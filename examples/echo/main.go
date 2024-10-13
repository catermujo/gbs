package main

import (
	"log"
	"net/http"

	"github.com/isinyaaa/gbs"
)

func main() {
	upgrader := gbs.NewUpgrader(&Handler{}, &gbs.ServerOption{
		CheckUtf8Enabled: true,
		Recovery:         gbs.Recovery,
	})
	http.HandleFunc("/connect", func(writer http.ResponseWriter, request *http.Request) {
		socket, err := upgrader.Upgrade(writer, request)
		if err != nil {
			return
		}
		go func() {
			socket.ReadLoop()
		}()
	})
	log.Panic(
		http.ListenAndServe(":8000", nil),
	)
}

type Handler struct {
	gbs.BuiltinEventHandler
}

func (c *Handler) OnPing(socket *gbs.Conn, payload []byte) {
	_ = socket.WritePong(payload)
}

func (c *Handler) OnMessage(socket *gbs.Conn, message *gbs.Message) {
	defer message.Close()
	_ = socket.WriteMessage(message.Opcode, message.Bytes())
}
