package main

import (
	"flag"
	"log"
	"path/filepath"

	"github.com/isinyaaa/gbs"
)

var dir string

func init() {
	flag.StringVar(&dir, "d", "", "cert directory")
	flag.Parse()

	d, err := filepath.Abs(dir)
	if err != nil {
		log.Printf(err.Error())
		return
	}
	dir = d
}

func main() {
	srv := gbs.NewServer(new(Websocket), nil)

	// wss://www.gbs.com:8443/
	if err := srv.RunTLS(":8443", dir+"/server.crt", dir+"/server.pem"); err != nil {
		log.Panicln(err.Error())
	}
}

type Websocket struct {
	gbs.BuiltinEventHandler
}

func (c *Websocket) OnPing(socket *gbs.Conn, payload []byte) {
	_ = socket.WritePong(payload)
}

func (c *Websocket) OnMessage(socket *gbs.Conn, message *gbs.Message) {
	defer message.Close()
	_ = socket.WriteMessage(message.Opcode, message.Bytes())
}
