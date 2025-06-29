package log

import (
	"testing"
	"time"
)

func TestLog(t *testing.T) {
	sub := Subscribe()
	go func() {
		for  v := range sub.Events() {
			t.Log(v.Level.String(), v.Message)
		}
	}()

	Infoln("aa: %v", 11 )
	Debugln("now is %v", time.Now())
	Errorln("success is %d", 111)
	Traceln("aaa")
	Warnln("java: %+v", 2222)

	time.Sleep(time.Second*2)
}
