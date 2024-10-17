package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/catermujo/gbs"
)

func main() {
	socket, _, err := gbs.NewClient(new(WebSocket), &gbs.ClientOption{
		Addr: "ws://127.0.0.1:3000/connect",
	})
	if err != nil {
		log.Print(err.Error())
		return
	}
	go socket.ReadLoop()

	for {
		text := ""
		fmt.Scanf("%s", &text)
		if strings.TrimSpace(text) == "" {
			continue
		}
		socket.WriteString(text)
	}
}

type WebSocket struct{}

func (c *WebSocket) OnClose(socket *gbs.Conn, err error) {
	fmt.Printf("onerror: err=%s\n", err.Error())
}

func (c *WebSocket) OnPong(socket *gbs.Conn, payload []byte) {
}

func (c *WebSocket) OnOpen(socket *gbs.Conn) {
	_ = socket.WriteString("hello, there is client")
}

func (c *WebSocket) OnPing(socket *gbs.Conn, payload []byte) {
	_ = socket.WritePong(payload)
}

func (c *WebSocket) OnMessage(socket *gbs.Conn, message *gbs.Message) {
	defer message.Close()
	fmt.Printf("recv: %s\n", message.Data.String())
}
