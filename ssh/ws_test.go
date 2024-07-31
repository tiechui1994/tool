package ssh

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
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

func client() {
	m := map[string]string{
		"www.baidu.com:443": "[2409:8c20:6:1d55:0:ff:b09c:7d77]:443",
		"page-us-qg0.pages.dev:443":"47.236.202.91:443",
		"overtcp.pages.dev":"[2606:4700:310c::ac42:2d1f]:443",
	}

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				v := addr
				if val, ok := m[addr]; ok {
					v = val
				}
				fmt.Println("DialContext:", v)
				return (&net.Dialer{}).DialContext(context.Background(), network, v)
			},
			DisableKeepAlives: true,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
		},
	}
	//req, _ := http.NewRequest("GET", "https://page-us-qg0.pages.dev/", nil)
	resp, err := client.Get("https://overtcp.pages.dev/check")
	if err != nil {
		fmt.Println(err)
		return
	}

	_,_ = io.Copy(os.Stdout, resp.Body)
	fmt.Println(resp.Status)
}

func TestXc(t *testing.T) {
	client()
}
