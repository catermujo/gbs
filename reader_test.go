package gbs

import (
	"bytes"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/catermujo/gbs/internal"
	"github.com/stretchr/testify/assert"
)

// 测试同步读
func TestReadSync(t *testing.T) {
	mu := &sync.Mutex{}
	var listA []string
	var listB []string
	const count = 1000
	wg := &sync.WaitGroup{}
	wg.Add(count)

	serverHandler := new(webSocketMocker)
	clientHandler := new(webSocketMocker)
	serverOption := &ServerOption{}
	clientOption := &ClientOption{}

	serverHandler.onMessage = func(socket *Conn, message *Message) {
		mu.Lock()
		listB = append(listB, message.Data.String())
		mu.Unlock()
		wg.Done()
	}

	server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
	go server.ReadLoop()
	go client.ReadLoop()

	for i := 0; i < count; i++ {
		n := internal.AlphabetNumeric.Intn(1024)
		message := internal.AlphabetNumeric.Generate(n)
		listA = append(listA, string(message))
		client.WriteAsync(OpcodeText, message, nil)
	}

	wg.Wait()
	assert.ElementsMatch(t, listA, listB)
}

//go:embed assets/read_test.json
var testdata []byte

type testRow struct {
	Title    string `json:"title"`
	Payload  string `json:"payload"`
	Expected struct {
		EventHandler  string `json:"event"`
		Reason string `json:"reason"`
		Code   uint16 `json:"code"`
	} `json:"expected"`
	Length int   `json:"length"`
	Fin    bool  `json:"fin"`
	Opcode uint8 `json:"opcode"`
	RSV2   bool  `json:"rsv2"`
}

func TestRead(t *testing.T) {
	as := assert.New(t)

	items := make([]testRow, 0)
	if err := json.Unmarshal(testdata, &items); err != nil {
		as.NoError(err)
		return
	}

	for _, item := range items {
		println(item.Title)
		var payload []byte
		if item.Payload == "" {
			payload = internal.AlphabetNumeric.Generate(item.Length)
		} else {
			p, err := hex.DecodeString(item.Payload)
			if err != nil {
				as.NoError(err)
				return
			}
			payload = p
		}

		wg := &sync.WaitGroup{}
		wg.Add(1)

		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		serverOption := &ServerOption{
			ParallelEnabled:     true,
			CheckUtf8Enabled:    false,
			ReadMaxPayloadSize:  1024 * 1024,
			WriteMaxPayloadSize: 16 * 1024 * 1024,
		}
		clientOption := &ClientOption{
			ParallelEnabled:     true,
			CheckUtf8Enabled:    true,
			ReadMaxPayloadSize:  1024 * 1024,
			WriteMaxPayloadSize: 1024 * 1024,
		}

		switch item.Expected.EventHandler {
		case "onMessage":
			clientHandler.onMessage = func(socket *Conn, message *Message) {
				as.Equal(string(payload), message.Data.String())
				wg.Done()
			}
		case "onPing":
			clientHandler.onPing = func(socket *Conn, d []byte) {
				as.Equal(string(payload), string(d))
				wg.Done()
			}
		case "onPong":
			clientHandler.onPong = func(socket *Conn, d []byte) {
				as.Equal(string(payload), string(d))
				wg.Done()
			}
		case "onClose":
			clientHandler.onClose = func(socket *Conn, err error) {
				if v, ok := err.(*CloseError); ok {
					println(v.Error())
				}
				as.Error(err)
				wg.Done()
			}
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go client.ReadLoop()
		go server.ReadLoop()

		if item.Fin {
			server.WriteAsync(Opcode(item.Opcode), testCloneBytes(payload), nil)
		} else {
			testWrite(server, false, Opcode(item.Opcode), testCloneBytes(payload))
		}
		wg.Wait()
	}
}

func TestSegments(t *testing.T) {
	as := assert.New(t)

	t.Run("valid segments", func(t *testing.T) {
		wg := &sync.WaitGroup{}
		wg.Add(1)

		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		serverOption := &ServerOption{}
		clientOption := &ClientOption{}

		s1 := internal.AlphabetNumeric.Generate(16)
		s2 := internal.AlphabetNumeric.Generate(16)
		serverHandler.onMessage = func(socket *Conn, message *Message) {
			as.Equal(string(s1)+string(s2), message.Data.String())
			wg.Done()
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		go func() {
			testWrite(client, false, OpcodeText, testCloneBytes(s1))
			testWrite(client, true, OpcodeContinuation, testCloneBytes(s2))
		}()
		wg.Wait()
	})

	t.Run("long segments", func(t *testing.T) {
		wg := &sync.WaitGroup{}
		wg.Add(1)

		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		serverOption := &ServerOption{ReadMaxPayloadSize: 16}
		clientOption := &ClientOption{}

		s1 := internal.AlphabetNumeric.Generate(16)
		s2 := internal.AlphabetNumeric.Generate(16)
		serverHandler.onClose = func(socket *Conn, err error) {
			as.Error(err)
			wg.Done()
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		go func() {
			testWrite(client, false, OpcodeText, testCloneBytes(s1))
			testWrite(client, true, OpcodeContinuation, testCloneBytes(s2))
		}()
		wg.Wait()
	})

	t.Run("invalid segments", func(t *testing.T) {
		wg := &sync.WaitGroup{}
		wg.Add(1)

		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		serverOption := &ServerOption{}
		clientOption := &ClientOption{}

		s1 := internal.AlphabetNumeric.Generate(16)
		s2 := internal.AlphabetNumeric.Generate(16)
		serverHandler.onClose = func(socket *Conn, err error) {
			as.Error(err)
			wg.Done()
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		go func() {
			testWrite(client, false, OpcodeText, testCloneBytes(s1))
			testWrite(client, true, OpcodeText, testCloneBytes(s2))
		}()
		wg.Wait()
	})
}

func TestMessage(t *testing.T) {
	msg := &Message{
		Opcode: OpcodeText,
		Data:   bytes.NewBufferString("1234"),
	}
	_, _ = msg.Read(make([]byte, 2))
	msg.Close()
}

func TestFrameHeader_Parse(t *testing.T) {
	t.Run("", func(t *testing.T) {
		s, c := net.Pipe()
		c.Close()
		fh := frameHeader{}
		_, err := fh.Parse(s)
		assert.Error(t, err)
	})

	t.Run("", func(t *testing.T) {
		s, c := net.Pipe()
		go func() {
			h := frameHeader{}
			h.GenerateHeader(false, true, OpcodeText, 500)
			c.Write(h[:2])
			c.Close()
		}()

		time.Sleep(100 * time.Millisecond)
		fh := frameHeader{}
		_, err := fh.Parse(s)
		assert.Error(t, err)
	})

	t.Run("", func(t *testing.T) {
		s, c := net.Pipe()
		go func() {
			h := frameHeader{}
			h.GenerateHeader(false, true, OpcodeText, 1024*1024)
			c.Write(h[:2])
			c.Close()
		}()

		time.Sleep(100 * time.Millisecond)
		fh := frameHeader{}
		_, err := fh.Parse(s)
		assert.Error(t, err)
	})

	t.Run("", func(t *testing.T) {
		s, c := net.Pipe()
		go func() {
			h := frameHeader{}
			h.GenerateHeader(false, true, OpcodeText, 1024*1024)
			c.Write(h[:10])
			c.Close()
		}()

		time.Sleep(100 * time.Millisecond)
		fh := frameHeader{}
		_, err := fh.Parse(s)
		assert.Error(t, err)
	})
}

func TestConn_ReadMessage(t *testing.T) {
	t.Run("", func(t *testing.T) {
		addr := ":" + nextPort()
		serverHandler := &webSocketMocker{}
		serverHandler.onOpen = func(socket *Conn) {
			p := []byte("123")
			frame, _ := socket.genFrame(OpcodePing, internal.Bytes(p), frameConfig{
				fin:           true,
				broadcast:     false,
				checkEncoding: socket.config.CheckUtf8Enabled,
			})
			socket.conn.Write(frame.Bytes()[:2])
			socket.conn.Close()
		}
		server := NewServer(serverHandler, nil)
		go server.Run(addr)

		time.Sleep(100 * time.Millisecond)
		client, _, err := NewClient(new(BuiltinEventHandler), &ClientOption{
			Addr: "ws://localhost" + addr,
		})
		assert.NoError(t, err)
		client.ReadLoop()
	})

	t.Run("", func(t *testing.T) {
		addr := ":" + nextPort()
		serverHandler := &webSocketMocker{}
		serverHandler.onOpen = func(socket *Conn) {
			p := []byte("123")
			frame, _ := socket.genFrame(OpcodeText, internal.Bytes(p), frameConfig{
				fin:           true,
				broadcast:     false,
				checkEncoding: false,
			})
			socket.conn.Write(frame.Bytes()[:2])
			socket.conn.Close()
		}
		server := NewServer(serverHandler, nil)
		go server.Run(addr)

		time.Sleep(100 * time.Millisecond)
		client, _, err := NewClient(new(BuiltinEventHandler), &ClientOption{
			Addr: "ws://localhost" + addr,
		})
		assert.NoError(t, err)
		client.ReadLoop()
	})
}
