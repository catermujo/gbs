package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/catermujo/gbs"
)

func main() {
	h := &Handler{conns: gbs.NewConcurrentMap[string, *gbs.Conn]()}

	upgrader := gbs.NewUpgrader(h, &gbs.ServerOption{})

	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		socket, err := upgrader.Upgrade(writer, request)
		if err != nil {
			log.Println(err.Error())
			return
		}
		websocketKey := request.Header.Get("Sec-WebSocket-Key")
		socket.Session().Store("websocketKey", websocketKey)
		h.conns.Store(websocketKey, socket)
		go func() {
			socket.ReadLoop()
		}()
	})

	go func() {
		if err := http.ListenAndServe(":8000", nil); err != nil {
			return
		}
	}()

	for {
		msg := ""
		if _, err := fmt.Scanf("%s\n", &msg); err != nil {
			log.Println(err.Error())
			return
		}
		h.Broadcast(msg)
	}
}

func getSession[T any](s gbs.SessionStorage, key string) (val T) {
	if v, ok := s.Load(key); ok {
		val, _ = v.(T)
	}
	return
}

type Handler struct {
	gbs.BuiltinEventHandler
	conns *gbs.ConcurrentMap[string, *gbs.Conn]
}

func (c *Handler) Broadcast(msg string) {
	b := gbs.NewBroadcaster(gbs.OpcodeText, []byte(msg))
	c.conns.Range(func(key string, conn *gbs.Conn) bool {
		_ = b.Broadcast(conn)
		return true
	})
	_ = b.Close()
}

func (c *Handler) OnClose(socket *gbs.Conn, err error) {
	websocketKey := getSession[string](socket.Session(), "websocketKey")
	c.conns.Delete(websocketKey)
}

func (c *Handler) OnMessage(socket *gbs.Conn, message *gbs.Message) {
	defer message.Close()
}
