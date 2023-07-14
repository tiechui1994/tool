package speech

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/tiechui1994/tool/util"
	"log"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

func getClient(token *oauth2.Token) *http.Client {
	config := &oauth2.Config{}
	return config.Client(context.Background(), token)
}

type EventDateTime struct {
	DateTime string `json:"dateTime,omitempty"`
	TimeZone string `json:"timeZone,omitempty"`
}

type Recurrence []string

type Event struct {
	Start      *EventDateTime
	End        *EventDateTime
	Title      string
	Recurrence Recurrence
	Body       json.RawMessage
	Request    struct {
		EventID string            `json:"eventID"`
		URL     string            `json:"url"`
		Method  string            `json:"method"`
		Headers map[string]string `json:"headers"`
	}
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

func DeleteEvents(token oauth2.Token, start, end, zone string) error {
	service, err := calendar.NewService(context.Background(),
		option.WithHTTPClient(getClient(&token)))
	if err != nil {
		return err
	}

	calendarId := "primary"
	list, err := service.Events.List(calendarId).
		TimeMin(start).TimeMax(end).TimeZone(zone).MaxResults(512).Do()
	if err != nil {
		return err
	}

	for _, item := range list.Items {
		if len(item.Recurrence) == 0 {
			_ = delEvent(token, []string{item.Id})
			continue
		}

		instances, err := service.Events.Instances(calendarId, item.Id).
			TimeMin(start).TimeMax(end).TimeZone(zone).MaxResults(512).Do()
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

func DeleteEvent(token oauth2.Token, eventID string) error {
	return delEvent(token, []string{eventID})
}

func delEvent(token oauth2.Token, eventIdList []string) error {
	service, err := calendar.NewService(context.Background(),
		option.WithHTTPClient(getClient(&token)))
	if err != nil {
		return err
	}

	calendarId := "primary"
	if len(eventIdList) == 0 {
		return service.Events.Delete(calendarId, eventIdList[0]).Do()
	}

	in := make(chan string, 2)
	out := make(chan struct{})
	go func() {
		for _, eventId := range eventIdList {
			in <- eventId
		}
		close(in)
	}()

	for i := 0; i < 2; i++ {
		go func() {
			defer func() {
				out <- struct{}{}
			}()
			eventId, ok := <-in
			if !ok {
				return
			}
			err = service.Events.Delete(calendarId, eventId).Do()
			if err != nil {
				log.Printf("DEL failed: %v", err)
			}
		}()
	}

	for i := 0; i < len(eventIdList); i++ {
		<-out
	}

	return nil
}

func InsertEvent(event Event, token oauth2.Token) (err error) {
	service, err := calendar.NewService(context.Background(),
		option.WithHTTPClient(getClient(&token)))
	if err != nil {
		return err
	}

	uid := hex.EncodeToString(util.MD5(fmt.Sprintf("%v", time.Now().UnixNano())))
	event.Request.EventID = uid
	request, _ := json.Marshal(event.Request)

	ev := &calendar.Event{
		Id:          uid,
		Summary:     event.Title,
		Description: base64.StdEncoding.EncodeToString(event.Body),
		Start: &calendar.EventDateTime{
			DateTime: event.Start.DateTime,
			TimeZone: event.Start.TimeZone,
		},
		End: &calendar.EventDateTime{
			DateTime: event.End.DateTime,
			TimeZone: event.End.TimeZone,
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
