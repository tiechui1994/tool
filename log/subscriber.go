package log

import (
	"sync"

	"github.com/sirupsen/logrus"
)

type LogEvent struct {
	Level   logrus.Level
	Message string
}

// BaseSubscriber 是一个通用的订阅者实现
type BaseSubscriber struct {
	id     string
	levels []logrus.Level
	events chan LogEvent
	mu     sync.Mutex
	closed bool
}

// NewBaseSubscriber 创建一个新的基本订阅者
func NewBaseSubscriber(id string, levels ...logrus.Level) *BaseSubscriber {
	return &BaseSubscriber{
		id:     id,
		levels: levels,
		events: make(chan LogEvent, 512), // 使用缓冲通道
	}
}

// uuid 返回订阅者的唯一ID
func (s *BaseSubscriber) uuid() string {
	return s.id
}

// filter 根据预设的日志级别过滤事件
func (s *BaseSubscriber) filter(event LogEvent) bool {
	if len(s.levels) == 0 { // 如果没有指定级别，则接收所有事件
		return true
	}
	for _, level := range s.levels {
		if event.Level <= level { // 接收等于或低于指定级别的事件
			return true
		}
	}
	return false
}

// Events 返回接收事件的通道
func (s *BaseSubscriber) Events() chan LogEvent {
	return s.events
}

// Close 关闭订阅者，释放资源
func (s *BaseSubscriber) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		close(s.events)
		s.closed = true
	}
}

// Subscriber 定义了订阅者的接口
type Subscriber interface {
	uuid() string // 返回订阅者的唯一ID
	filter(event LogEvent) bool // 决定是否接收此事件
	Events() chan LogEvent // 返回接收事件的通道
	Close() // 关闭订阅者，释放资源
}

// SubscriberHook 是一个 Logrus Hook，负责将日志事件分发给订阅者
type SubscriberHook struct {
	subscribers map[string]Subscriber
	mu          sync.RWMutex
}

// NewSubscriberHook 创建一个新的 SubscriberHook
func NewSubscriberHook() *SubscriberHook {
	return &SubscriberHook{
		subscribers: make(map[string]Subscriber),
	}
}

// AddSubscriber 添加一个订阅者
func (h *SubscriberHook) AddSubscriber(sub Subscriber) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.subscribers[sub.uuid()] = sub
}

// RemoveSubscriber 移除一个订阅者
func (h *SubscriberHook) RemoveSubscriber(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if sub, ok := h.subscribers[id]; ok {
		sub.Close() // 关闭订阅者的通道
		delete(h.subscribers, id)
	}
}

// Levels 返回 Hook 应该处理的日志级别
func (h *SubscriberHook) Levels() []logrus.Level {
	return logrus.AllLevels // 处理所有日志级别
}

// Fire 是 Hook 的核心方法，当有日志事件时被调用
func (h *SubscriberHook) Fire(entry *logrus.Entry) error {
	event := LogEvent{
		Level:   entry.Level,
		Message: entry.Message,
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, sub := range h.subscribers {
		// 使用 goroutine 非阻塞地发送事件，避免阻塞日志写入
		go func(s Subscriber) {
			if s.filter(event) {
				select {
				case s.Events() <- event:
					// 事件发送成功
				default:
					// 通道已满，丢弃事件，或者可以实现更复杂的重试/错误处理机制
					logrus.Warnf("Subscriber %s channel is full, dropping event: %s", s.uuid(), event.Message)
				}
			}
		}(sub)
	}
	return nil
}