package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/thiagozilli-mb/go-websocketclient/models"
)

type Scheme int

const (
	WS Scheme = iota
	WSS
)

func (s Scheme) String() string {
	return [...]string{"ws", "wss"}[s]
}

const pingPeriod = 30 * time.Second

type WebSocketClient struct {
	configStr string
	sendBuf   chan []byte
	ctx       context.Context
	ctxCancel context.CancelFunc
	mu        sync.RWMutex
	wsconn    *websocket.Conn
}

func NewWebSocketClient(scheme Scheme, host, path string) (*WebSocketClient, error) {
	conn := WebSocketClient{
		sendBuf: make(chan []byte, 1),
	}
	conn.ctx, conn.ctxCancel = context.WithCancel(context.Background())

	u := url.URL{Scheme: scheme.String(), Host: host, Path: path}
	conn.configStr = u.String()

	go conn.listen()
	go conn.listenWrite()
	go conn.ping()
	return &conn, nil
}

func (conn *WebSocketClient) Subscribe(channel, pair string) error {
	sub := models.Subscribe{
		Type: "subscribe",
		Subscription: models.Subscription{
			Name: channel,
			ID:   pair,
		},
	}
	if channel == "orderbook" {
		sub.Subscription.Limit = 10
	}
	return conn.Write(sub)
}

func (conn *WebSocketClient) Connect() *websocket.Conn {
	conn.mu.Lock()
	defer conn.mu.Unlock()
	if conn.wsconn != nil {
		return conn.wsconn
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for ; ; <-ticker.C {
		select {
		case <-conn.ctx.Done():
			return nil
		default:
			header := make(http.Header)
			header.Add("Origin", "http://localhost")
			header.Add(
				"User-Agent",
				"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_14_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/78.0.3904.108 Safari/537.36",
			)

			ws, resp, err := websocket.DefaultDialer.Dial(conn.configStr, header)
			if err == websocket.ErrBadHandshake {
				conn.log("handshake", err, fmt.Sprintf("handshake failed with status %d", resp.StatusCode))
			}
			if err != nil {
				conn.log("connect", err, fmt.Sprintf("Cannot connect to websocket: %s", conn.configStr))
				continue
			}
			conn.log("connect", nil, fmt.Sprintf("connected to websocket to %s", conn.configStr))

			conn.wsconn = ws
			return conn.wsconn
		}
	}
}

func (conn *WebSocketClient) listen() {
	conn.log("listen", nil, fmt.Sprintf("listen for the messages: %s", conn.configStr))
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-conn.ctx.Done():
			return
		case <-ticker.C:
			for {
				ws := conn.Connect()
				if ws == nil {
					return
				}

				_, bytMsg, err := ws.ReadMessage()
				if err != nil {
					conn.log("listen", err, "Cannot read websocket message")
					conn.closeWS()
					break
				}
				conn.log("listen", nil, fmt.Sprintf("websocket msg: %s\n", bytMsg))
			}
		}
	}
}

func (conn *WebSocketClient) Write(payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*50)
	defer cancel()

	for {
		select {
		case conn.sendBuf <- data:
			return nil
		case <-ctx.Done():
			return fmt.Errorf("context canceled")
		}
	}
}

func (conn *WebSocketClient) listenWrite() {
	for data := range conn.sendBuf {
		ws := conn.Connect()
		if ws == nil {
			err := fmt.Errorf("conn ws is nil")
			conn.log("listenWrite", err, "No websocket connection")
			continue
		}

		if err := ws.WriteMessage(
			websocket.TextMessage,
			data,
		); err != nil {
			conn.log("listenWrite", nil, "WebSocket Write Error")
		}
		conn.log("listenWrite", nil, fmt.Sprintf("send: %s", data))
	}
}

func (conn *WebSocketClient) Stop() {
	conn.ctxCancel()
	conn.closeWS()
}

func (conn *WebSocketClient) closeWS() {
	conn.mu.Lock()
	defer conn.mu.Unlock()
	if conn.wsconn != nil {
		conn.wsconn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		conn.wsconn.Close()
		conn.wsconn = nil
	}
}

func (conn *WebSocketClient) ping() {
	conn.log("ping", nil, "ping pong started")
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			ws := conn.Connect()
			if ws == nil {
				continue
			}
			if err := conn.wsconn.WriteControl(websocket.PingMessage,
				[]byte{}, time.Now().Add(pingPeriod/2)); err != nil {
				conn.closeWS()
			}
		case <-conn.ctx.Done():
			return
		}
	}
}

func (conn *WebSocketClient) log(f string, err error, msg string) {
	if err != nil {
		fmt.Printf("Error in func: %s, err: %v, msg: %s\n", f, err, msg)
	} else {
		fmt.Printf("Log in func: %s, %s\n", f, msg)
	}
}
