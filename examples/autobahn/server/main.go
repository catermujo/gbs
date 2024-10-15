package main

import (
	"log"

	"github.com/isinyaaa/gbs"
)

func main() {
	s1 := gbs.NewServer(&Handler{Sync: true}, &gbs.ServerOption{
		CheckUtf8Enabled: true,
		Recovery:         gbs.Recovery,
	})

	s2 := gbs.NewServer(&Handler{Sync: false}, &gbs.ServerOption{
		ParallelEnabled:  true,
		CheckUtf8Enabled: true,
		Recovery:         gbs.Recovery,
	})

	s3 := gbs.NewServer(&Handler{Sync: true}, &gbs.ServerOption{
		CheckUtf8Enabled: true,
		Recovery:         gbs.Recovery,
	})

	s4 := gbs.NewServer(&Handler{Sync: false}, &gbs.ServerOption{
		ParallelEnabled:  true,
		CheckUtf8Enabled: true,
		Recovery:         gbs.Recovery,
	})

	go func() {
		log.Panic(s1.Run(":8000"))
	}()

	go func() {
		log.Panic(s2.Run(":8001"))
	}()

	go func() {
		log.Panic(s3.Run(":8002"))
	}()

	log.Panic(s4.Run(":8003"))
}

type Handler struct {
	gbs.BuiltinEventHandler
	Sync bool
}

func (c *Handler) OnPing(socket *gbs.Conn, payload []byte) {
	_ = socket.WritePong(payload)
}

func (c *Handler) OnMessage(socket *gbs.Conn, message *gbs.Message) {
	if c.Sync {
		_ = socket.WriteMessage(message.Opcode, message.Bytes())
		_ = message.Close()
	} else {
		socket.WriteAsync(message.Opcode, message.Bytes(), func(err error) { _ = message.Close() })
	}
}
