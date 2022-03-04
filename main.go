package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/thiagozilli-mb/go-websocketclient/pkg/client"
)

var addr = flag.String("addr", ":3333", "http service address")
var path = flag.String("path", "ws", "path of websocket")

func main() {
	flag.Parse()

	client, err := client.NewWebSocketClient(client.WSS, *addr, *path)
	if err != nil {
		panic(err)
	}

	if err := Run(client, *addr, *path); err != nil {
		panic(err)
	}
}

func Run(c *client.WebSocketClient, addr, path string) error {

	fmt.Println("Client Running")

	pairs := make(map[string]string)
	pairs["ticker"] = "BRLBCH"
	// pairs["orderbook"] = "BRLBTC"

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	Subscribe(c, pairs)

	<-sigs
	Shutdown(c, pairs)

	return nil
}

func Shutdown(c *client.WebSocketClient, pairs map[string]string) {
	for channel, pair := range pairs {
		c.Subscribe(channel, pair)
	}
	c.Stop()
	fmt.Println("Client shutdown")
}

func Subscribe(c *client.WebSocketClient, pairs map[string]string) {
	for channel, pair := range pairs {
		c.Subscribe(channel, pair)
	}
}
