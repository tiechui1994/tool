package speech

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/tiechui1994/tool/util"
	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

const (
	timeFormat   = "2006-01-02T15:04:05+08:00"
	ZoneShanghai = "Asia/Shanghai"
)

func getClient(token *Token) *http.Client {
	config := &oauth2.Config{}
	return config.Client(context.Background(), &oauth2.Token{
		AccessToken: token.AccessToken,
		RefreshToken: token.RefreshToken,
		TokenType: token.TokenType,
		Expiry: token.Expiry,
	})
}

type EventDateTime struct {
	DateTime time.Time `json:"dateTime,omitempty"`
	TimeZone string    `json:"timeZone,omitempty"`
}

type Recurrence []string
type Request struct {
	EventID string            `json:"eventID"`
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
}

type Event struct {
	TimeAt     *time.Time
	TimeZone   string
	Title      string
	Recurrence Recurrence
	Body       json.RawMessage
	Request    Request
}

type Token struct {
	AccessToken string `json:"access_token"`
	TokenType string `json:"token_type,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Expiry time.Time `json:"expiry,omitempty"`
}

type EventOption interface {
	apply() []string
}

type eventOption struct {
	f func() []string
}

func (o *eventOption) apply() []string {
	return o.f()
}

func newEventOption(f func() []string) *eventOption {
	return &eventOption{f: f}
}

type Cron struct {
	Minute []int
	Hour   []int
}

// UNTIL=20110701T170000Z
// BYHOUR=
// BYMINUTE=
// BYSECOND=
//
// INTERVAL=2, 每间隔的时长
// COUNT=10, 总共10次
// WKST=MO, 每一天
func withCron(cron Cron, callback func() string) *eventOption {
	return newEventOption(func() []string {
		if len(cron.Hour) == 0 {
			cron.Hour = []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23}
		}

		var result []string
		for _, hour := range cron.Hour {
			for _, min := range cron.Minute {
				v := fmt.Sprintf("RRULE:FREQ=DAILY;BYHOUR=%v;BYMINUTE=%v;%v", hour, min, callback())
				result = append(result, v)
			}
		}

		return result
	})
}

func WithEmpty() Recurrence {
	return newEventOption(func() []string {
		return []string{}
	}).apply()
}

func WithCron(c Cron, interval ...int) Recurrence {
	interval = append(interval, 1)
	return withCron(c, func() string {
		return fmt.Sprintf("COUNT=1;INTERVAL=%v", interval[0])
	}).apply()
}

func WithCount(c Cron, count int, interval ...int) Recurrence {
	interval = append(interval, 1)
	return withCron(c, func() string {
		return fmt.Sprintf("COUNT=%v;INTERVAL=%v", count, interval[0])
	}).apply()
}

func WithUntil(c Cron, until time.Time, interval ...int) Recurrence {
	interval = append(interval, 1)
	return withCron(c, func() string {
		return fmt.Sprintf("UNTIL=%v;INTERVAL=%v", until.Format("20060102T150405Z"), interval[0])
	}).apply()
}

func WithForever(c Cron, interval ...int) Recurrence {
	interval = append(interval, 1)
	return withCron(c, func() string {
		return fmt.Sprintf("WKST=MO;INTERVAL=%v", interval[0])
	}).apply()
}

func DeleteEvents(token Token, start, end string) error {
	service, err := calendar.NewService(context.Background(),
		option.WithHTTPClient(getClient(&token)))
	if err != nil {
		return err
	}

	calendarId := "primary"
	list, err := service.Events.List(calendarId).
		TimeMin(start).TimeMax(end).TimeZone(ZoneShanghai).MaxResults(512).Do()
	if err != nil {
		return err
	}

	for _, item := range list.Items {
		if len(item.Recurrence) == 0 {
			_ = delEvent(token, []string{item.Id})
			continue
		}

		instances, err := service.Events.Instances(calendarId, item.Id).
			TimeMin(start).TimeMax(end).TimeZone(ZoneShanghai).MaxResults(512).Do()
		if err != nil {
			continue
		}

		eventIdList := make([]string, 0, len(instances.Items))
		for _, it := range instances.Items {
			eventIdList = append(eventIdList, it.Id)
		}
		_ = delEvent(token, eventIdList)
	}

	return err
}

func DeleteEvent(token Token, eventID string) error {
	service, err := calendar.NewService(context.Background(),
		option.WithHTTPClient(getClient(&token)))
	if err != nil {
		return err
	}

	calendarId := "primary"
	instances, err := service.Events.Instances(calendarId, eventID).
		MaxResults(512).Do()
	if err != nil {
		return err
	}

	if len(instances.Items) == 0 {
		return delEvent(token, []string{eventID})
	}

	zone, err := time.LoadLocation(ZoneShanghai)
	if err != nil {
		return err
	}

	eventIdList := make([]string, 0)
	now := time.Now().In(zone).Format(timeFormat)
	for _, item := range instances.Items {
		if item.Start.DateTime < now {
			eventIdList = append(eventIdList, item.Id)
		}
	}

	return delEvent(token, eventIdList)
}

func delEvent(token Token, eventIdList []string) error {
	service, err := calendar.NewService(context.Background(),
		option.WithHTTPClient(getClient(&token)))
	if err != nil {
		return err
	}

	calendarId := "primary"
	if len(eventIdList) == 1 {
		return service.Events.Delete(calendarId, eventIdList[0]).Do()
	}

	in := make(chan string, 2)
	wg := sync.WaitGroup{}
	go func() {
		defer close(in)
		for _, eventId := range eventIdList {
			in <- eventId
		}
	}()

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				eventId, ok := <-in
				if !ok {
					return
				}

				err = service.Events.Delete(calendarId, eventId).Do()
				if err != nil {
					log.Printf("DEL failed: %v", err)
				}
			}
		}()
	}

	wg.Wait()
	return nil
}

func InsertEvent(event Event, token Token) (err error) {
	if event.TimeAt == nil {
		return fmt.Errorf("attr TimeAt must be set")
	}

	service, err := calendar.NewService(context.Background(),
		option.WithHTTPClient(getClient(&token)))
	if err != nil {
		return err
	}

	if event.TimeZone == "" {
		event.TimeZone = ZoneShanghai
	}
	if event.Title == "" {
		event.Title = "Event_" + time.Now().Format("0102150405")
	}

	uid := hex.EncodeToString(util.MD5(fmt.Sprintf("%v", time.Now().UnixNano())))
	event.Request.EventID = uid
	request, _ := json.Marshal(event.Request)

	zone, err := time.LoadLocation(event.TimeZone)
	if err != nil {
		return err
	}
	at := event.TimeAt.In(zone)

	ev := &calendar.Event{
		Id:          uid,
		Summary:     event.Title,
		Description: base64.StdEncoding.EncodeToString(event.Body),
		Start: &calendar.EventDateTime{
			DateTime: at.Format(timeFormat),
			TimeZone: event.TimeZone,
		},
		End: &calendar.EventDateTime{
			DateTime: at.Add(time.Minute).Format(timeFormat),
			TimeZone: event.TimeZone,
		},
		Recurrence: event.Recurrence,
		Location:   base64.StdEncoding.EncodeToString(request),
		Reminders: &calendar.EventReminders{
			Overrides: []*calendar.EventReminder{
				{
					Method:          "email",
					Minutes:         0,
					ForceSendFields: []string{"Minutes"},
				},
			},
			UseDefault:      false,
			ForceSendFields: []string{"UseDefault"},
		},
	}

	calendarId := "primary"
	ev, err = service.Events.Insert(calendarId, ev).Do()
	if err != nil {
		return err
	}

	return nil
}
