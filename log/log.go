package log

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	hook =  NewSubscriberHook()
)

func init() {
	logrus.SetOutput(NewTimeoutWriter(time.Second))
	logrus.SetLevel(InfoLevel)
}

func Traceln(format string, args ...interface{})  {
	sprint(TraceLevel, fmt.Sprintf(format, args...))
	logrus.Tracef(format, args...)
}

func Debugln(format string, args ...interface{})  {
	sprint(DebugLevel, fmt.Sprintf(format, args...))
	logrus.Debugf(format, args...)
}

func Infoln(format string, args ...interface{})  {
	sprint(InfoLevel, fmt.Sprintf(format, args...))
	logrus.Infof(format, args...)
}

func Warnln(format string, args ...interface{})  {
	sprint(WarnLevel, fmt.Sprintf(format, args...))
	logrus.Warnf(format, args...)
}

func Errorln(format string, args ...interface{})  {
	sprint(ErrorLevel, fmt.Sprintf(format, args...))
	logrus.Errorf(format, args...)
}

func Fatalln(format string, args ...interface{})  {
	sprint(FatalLevel, fmt.Sprintf(format, args...))
	logrus.Fatalf(format, args...)
}

func sprint(level Level, message string)  {
	hook.Fire(&logrus.Entry{
		Level: level,
		Message: message,
	})
}

func SetOutput(out io.Writer)  {
	logrus.SetOutput(out)
}

func GetLevel() logrus.Level {
	return logrus.GetLevel()
}

func SetLevel(newLevel logrus.Level) {
	logrus.SetLevel(newLevel)
}

func Subscribe() Subscriber {
	id := time.Now().Format(time.RFC3339Nano)
	sub := NewBaseSubscriber(id)
	hook.AddSubscriber(sub)
	return sub
}

func UnSubscribe(sub Subscriber) {
	if hook != nil {
		hook.RemoveSubscriber(sub.uuid())
	}
}


type stdTimeoutWriter struct {
	timeout time.Duration
}

func NewTimeoutWriter(timeout time.Duration) io.Writer  {
	return &stdTimeoutWriter{timeout: timeout}
}

func (w *stdTimeoutWriter) Write(p []byte) (int, error)  {
	ctx, cancel := context.WithTimeout(context.Background(), w.timeout)
	defer cancel()

	done := make(chan struct{n int; err error})
	go func() {
		var  result struct {
			n   int
			err error
		}
		result.n, result.err = os.Stdout.Write(p)
		done <- result
	}()

	select {
	case <-ctx.Done():
		return 0, fmt.Errorf("write timeout")
	case val := <-done:
		return val.n, val.err
	}
}

