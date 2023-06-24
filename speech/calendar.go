package speech

import (
	"context"
	"fmt"
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

type EventDateTime = calendar.EventDateTime

type Event struct {
	Start       *EventDateTime
	End         *EventDateTime
	Description string
	Summary     string
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

func WithCron(c Cron, interval ...int) *eventOption {
	interval = append(interval, 1)
	return withCron(c, func() string {
		return fmt.Sprintf("COUNT=1;INTERVAL=%v", interval[0])
	})
}

func WithCount(c Cron, count int, interval ...int) *eventOption {
	interval = append(interval, 1)
	return withCron(c, func() string {
		return fmt.Sprintf("COUNT=%v;INTERVAL=%v", count, interval[0])
	})
}

func WithUntil(c Cron, until time.Time, interval ...int) *eventOption {
	interval = append(interval, 1)
	return withCron(c, func() string {
		return fmt.Sprintf("UNTIL=%v;INTERVAL=%v", until.Format("20060102T150405Z"), interval[0])
	})
}

func WithForever(c Cron, interval ...int) *eventOption {
	interval = append(interval, 1)
	return withCron(c, func() string {
		return fmt.Sprintf("WKST=MO;INTERVAL=%v", interval[0])
	})
}

func GetEvent(token oauth2.Token) error {
	service, err := calendar.NewService(context.Background(),
		option.WithHTTPClient(getClient(&token)))
	if err != nil {
		return err
	}

	calendarId := "primary"
	list, err := service.Events.List(calendarId).Do()
	if err != nil {
		return err
	}

	for _, v := range list.Items {
		raw, _ := v.MarshalJSON()
		fmt.Println(string(raw))
	}

	return err
}

func InsertEvent(event Event, token oauth2.Token, frequency EventOption) (err error) {
	service, err := calendar.NewService(context.Background(),
		option.WithHTTPClient(getClient(&token)))
	if err != nil {
		return err
	}

	ev := &calendar.Event{
		Summary:     event.Summary,
		Description: event.Description,
		Start:       event.Start,
		End:         event.End,
		Recurrence:  frequency.apply(),
	}

	calendarId := "primary"
	ev, err = service.Events.Insert(calendarId, ev).Do()
	if err != nil {
		return err
	}

	raw, err := ev.MarshalJSON()
	fmt.Println(string(raw), err)
	return
}
