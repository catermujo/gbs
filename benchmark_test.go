package gws

import (
	"bufio"
	"bytes"
	_ "embed"
	"encoding/binary"
	"net"
	"testing"

	"github.com/lxzan/gws/internal"
)

//go:embed assets/github.json
var githubData []byte

type benchConn struct {
	net.TCPConn
}

func (m benchConn) Write(p []byte) (n int, err error) {
	return len(p), nil
}

func BenchmarkConn_WriteMessage(b *testing.B) {
	b.Run("", func(b *testing.B) {
		upgrader := NewUpgrader(&BuiltinEventHandler{}, nil)
		conn := &Conn{
			conn:   &benchConn{},
			config: upgrader.option.getConfig(),
		}
		for i := 0; i < b.N; i++ {
			_ = conn.WriteMessage(OpcodeText, githubData)
		}
	})
}

func BenchmarkConn_ReadMessage(b *testing.B) {
	handler := &webSocketMocker{}
	handler.onMessage = func(socket *Conn, message *Message) { _ = message.Close() }

	b.Run("", func(b *testing.B) {
		upgrader := NewUpgrader(handler, nil)
		conn1 := &Conn{
			isServer: false,
			conn:     &benchConn{},
			config:   upgrader.option.getConfig(),
		}
		buf, _ := conn1.genFrame(OpcodeText, internal.Bytes(githubData), frameConfig{
			fin:           true,
			broadcast:     false,
			checkEncoding: false,
		})

		reader := bytes.NewBuffer(buf.Bytes())
		conn2 := &Conn{
			isServer: true,
			conn:     &benchConn{},
			br:       bufio.NewReader(reader),
			config:   upgrader.option.getConfig(),
			handler:  upgrader.eventHandler,
		}
		for i := 0; i < b.N; i++ {
			internal.BufferReset(reader, buf.Bytes())
			conn2.br.Reset(reader)
			_ = conn2.readMessage()
		}
	})
}

func BenchmarkMask(b *testing.B) {
	s1 := internal.AlphabetNumeric.Generate(1280)
	s2 := s1
	var key [4]byte
	binary.LittleEndian.PutUint32(key[:4], internal.AlphabetNumeric.Uint32())
	for i := 0; i < b.N; i++ {
		internal.MaskXOR(s2, key[:4])
	}
}

func BenchmarkConcurrentMap_ReadWrite(b *testing.B) {
	const count = 1000000
	cm := NewConcurrentMap[string, uint8](64)
	keys := make([]string, 0, count)
	for i := 0; i < count; i++ {
		key := string(internal.AlphabetNumeric.Generate(16))
		keys = append(keys, key)
		cm.Store(key, 1)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			i++
			key := keys[i%count]
			if i&15 == 0 {
				cm.Store(key, 1)
			} else {
				cm.Load(key)
			}
		}
	})
}
