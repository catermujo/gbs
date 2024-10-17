package main

import (
	"fmt"
	"log"
	"time"

	"github.com/catermujo/gbs"
)

const remoteAddr = "127.0.0.1:9001"

func main() {
	const count = 517
	for i := 1; i <= count; i++ {
		testCase(true, i, "gbs-client/sync")
	}
	for i := 1; i <= count; i++ {
		testCase(false, i, "gbs-client/async")
	}
	updateReports()
}

func testCase(sync bool, id int, agent string) {
	url := fmt.Sprintf("ws://%s/runCase?case=%d&agent=%s", remoteAddr, id, agent)
	handler := &WebSocket{Sync: sync, onexit: make(chan struct{})}
	socket, _, err := gbs.NewClient(handler, &gbs.ClientOption{
		Addr:             url,
		CheckUtf8Enabled: true,
	})
	if err != nil {
		log.Println(err.Error())
		return
	}
	go socket.ReadLoop()
	<-handler.onexit
}

type WebSocket struct {
	onexit chan struct{}
	Sync   bool
}

func (c *WebSocket) OnOpen(socket *gbs.Conn) {
	_ = socket.SetDeadline(time.Now().Add(30 * time.Second))
}

func (c *WebSocket) OnClose(socket *gbs.Conn, err error) {
	c.onexit <- struct{}{}
}

func (c *WebSocket) OnPing(socket *gbs.Conn, payload []byte) {
	_ = socket.WritePong(payload)
}

func (c *WebSocket) OnPong(socket *gbs.Conn, payload []byte) {}

func (c *WebSocket) OnMessage(socket *gbs.Conn, message *gbs.Message) {
	if c.Sync {
		_ = socket.WriteMessage(message.Opcode, message.Bytes())
		_ = message.Close()
	} else {
		socket.WriteAsync(message.Opcode, message.Bytes(), func(err error) { _ = message.Close() })
	}
}

type updateReportsHandler struct {
	gbs.BuiltinEventHandler
	onexit chan struct{}
}

func (c *updateReportsHandler) OnOpen(socket *gbs.Conn) {
	_ = socket.SetDeadline(time.Now().Add(5 * time.Second))
}

func (c *updateReportsHandler) OnClose(socket *gbs.Conn, err error) {
	c.onexit <- struct{}{}
}

func updateReports() {
	url := fmt.Sprintf("ws://%s/updateReports?agent=gbs/client", remoteAddr)
	handler := &updateReportsHandler{onexit: make(chan struct{})}
	socket, _, err := gbs.NewClient(handler, &gbs.ClientOption{
		Addr:             url,
		CheckUtf8Enabled: true,
	})
	if err != nil {
		log.Println(err.Error())
		return
	}
	go socket.ReadLoop()
	<-handler.onexit
}
