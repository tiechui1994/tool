package ssh

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func connect() error {
	u := "ws://localhost:8000/_/ws?rule=manage"
	conn, resp, err := websocket.DefaultDialer.DialContext(context.Background(), u, nil)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusSwitchingProtocols {
		buf := bytes.NewBuffer(nil)
		_ = resp.Write(buf)
		return fmt.Errorf("statusCode != 101:\n%s", buf.String())
	}

	done := make(chan struct{})
	conn.SetPongHandler(func(appData string) error {
		fmt.Println("=========== PONG ===============")
		return nil
	})

	go func() {
		for {
			select {
			case <-done:
				return
			default:
				_type, v, err := conn.ReadMessage()
				if err != nil {
					log.Printf("ReadMessage: %v", err)
					close(done)
					return
				}

				fmt.Println("Revive:", string(v), _type)
			}
		}
	}()

	for {
		select {
		case <-done:
			return nil
		default:
			err := conn.WriteMessage(websocket.PingMessage, []byte(nil))
			if err != nil {
				log.Printf("WriteMessage: %v", err)
				close(done)
				return nil
			}
			//
			//// Mon Jul 29 2024 06:19:06
			//err = conn.WriteMessage(websocket.TextMessage, []byte(time.Now().Format("2006-01-02T15:04:05.99999Z")))
			//if err != nil {
			//	log.Printf("WriteMessage: %v", err)
			//	close(done)
			//	return nil
			//}

			time.Sleep(time.Millisecond * 300)
		}
	}
}

func TestClient(t *testing.T) {
	t.Log(connect())
}
