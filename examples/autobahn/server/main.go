package main

import (
	"log"

	"github.com/lxzan/gws"
)

func main() {
	s1 := gws.NewServer(&Handler{Sync: true}, &gws.ServerOption{
		CheckUtf8Enabled: true,
		Recovery:         gws.Recovery,
	})

	s2 := gws.NewServer(&Handler{Sync: false}, &gws.ServerOption{
		ParallelEnabled:  true,
		CheckUtf8Enabled: true,
		Recovery:         gws.Recovery,
	})

	s3 := gws.NewServer(&Handler{Sync: true}, &gws.ServerOption{
		CheckUtf8Enabled: true,
		Recovery:         gws.Recovery,
	})

	s4 := gws.NewServer(&Handler{Sync: false}, &gws.ServerOption{
		ParallelEnabled:  true,
		CheckUtf8Enabled: true,
		Recovery:         gws.Recovery,
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
	gws.BuiltinEventHandler
	Sync bool
}

func (c *Handler) OnPing(socket *gws.Conn, payload []byte) {
	_ = socket.WritePong(payload)
}

func (c *Handler) OnMessage(socket *gws.Conn, message *gws.Message) {
	if c.Sync {
		_ = socket.WriteMessage(message.Opcode, message.Bytes())
		_ = message.Close()
	} else {
		socket.WriteAsync(message.Opcode, message.Bytes(), func(err error) { _ = message.Close() })
	}
}
