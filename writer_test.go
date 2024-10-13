package gbs

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/isinyaaa/gbs/internal"
	"github.com/stretchr/testify/assert"
)

func testWrite(c *Conn, fin bool, opcode Opcode, payload []byte) error {
	if len(payload) > c.config.WriteMaxPayloadSize {
		return internal.CloseMessageTooLarge
	}

	header := frameHeader{}
	n := len(payload)
	headerLength, maskBytes := header.GenerateHeader(c.isServer, fin, opcode, n)
	if !c.isServer {
		internal.MaskXOR(payload, maskBytes)
	}

	buf := make(net.Buffers, 0, 2)
	buf = append(buf, header[:headerLength])
	if n > 0 {
		buf = append(buf, payload)
	}
	num, err := buf.WriteTo(c.conn)
	if err != nil {
		return err
	}
	if int(num) < headerLength+n {
		return io.ErrShortWrite
	}
	return nil
}

func TestWriteClose(t *testing.T) {
	as := assert.New(t)

	t.Run("", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		serverOption := &ServerOption{}
		clientOption := &ClientOption{}

		wg := sync.WaitGroup{}
		wg.Add(1)
		serverHandler.onClose = func(socket *Conn, err error) {
			as.Error(err)
			wg.Done()
		}
		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()
		server.WriteClose(1000, []byte("goodbye"))
		wg.Wait()
		socket := &Conn{closed: 1, config: server.config}
		socket.WriteMessage(OpcodeText, nil)
		socket.WriteAsync(OpcodeText, nil, nil)
	})

	t.Run("", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		serverOption := &ServerOption{}
		clientOption := &ClientOption{}
		wg := &sync.WaitGroup{}
		wg.Add(1)

		serverHandler.onClose = func(socket *Conn, err error) {
			if v, ok := err.(*CloseError); ok && string(v.Reason) == "goodbye" {
				wg.Done()
			}
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		err := client.WriteClose(1006, []byte("goodbye"))
		assert.NoError(t, err)
		err = client.WriteClose(1006, []byte("goodbye"))
		assert.True(t, errors.Is(err, ErrConnClosed))
		wg.Wait()
	})

	t.Run("", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		serverOption := &ServerOption{}
		clientOption := &ClientOption{}
		wg := &sync.WaitGroup{}
		wg.Add(1)

		serverHandler.onClose = func(socket *Conn, err error) {
			if v, ok := err.(*CloseError); ok && len(v.Reason) == 123 {
				wg.Done()
			}
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		err := client.WriteClose(1006, internal.AlphabetNumeric.Generate(1024))
		assert.NoError(t, err)
		wg.Wait()
	})
}

func TestConn_WriteAsyncError(t *testing.T) {
	t.Run("write async", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		serverOption := &ServerOption{}
		clientOption := &ClientOption{}
		server, _ := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		server.closed = 1
		server.WriteAsync(OpcodeText, nil, nil)
	})

	t.Run("", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		serverOption := &ServerOption{CheckUtf8Enabled: true}
		clientOption := &ClientOption{}
		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go client.ReadLoop()

		// deflate压缩算法的尾部标记
		// The tail marker of the deflate compression algorithm
		flateTail := []byte{0x00, 0x00, 0xff, 0xff, 0x01, 0x00, 0x00, 0xff, 0xff}
		server.WriteAsync(OpcodeText, flateTail, func(err error) {
			assert.Error(t, err)
		})
	})
}

func TestConn_WriteInvalidUTF8(t *testing.T) {
	as := assert.New(t)
	serverHandler := new(webSocketMocker)
	clientHandler := new(webSocketMocker)
	serverOption := &ServerOption{CheckUtf8Enabled: true}
	clientOption := &ClientOption{}
	server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
	go server.ReadLoop()
	go client.ReadLoop()
	payload := []byte{1, 2, 255}
	as.Error(server.WriteMessage(OpcodeText, payload))
}

func TestConn_WriteClose(t *testing.T) {
	wg := sync.WaitGroup{}
	wg.Add(3)
	serverHandler := new(webSocketMocker)
	clientHandler := new(webSocketMocker)
	serverOption := &ServerOption{CheckUtf8Enabled: true}
	clientOption := &ClientOption{}
	server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
	clientHandler.onClose = func(socket *Conn, err error) {
		wg.Done()
	}
	clientHandler.onMessage = func(socket *Conn, message *Message) {
		wg.Done()
	}
	go server.ReadLoop()
	go client.ReadLoop()

	server.WriteMessage(OpcodeText, nil)
	server.WriteMessage(OpcodeText, []byte("hello"))
	server.WriteMessage(OpcodeCloseConnection, []byte{1})
	wg.Wait()
}

func TestNewBroadcaster(t *testing.T) {
	as := assert.New(t)

	t.Run("", func(t *testing.T) {
		handler := &broadcastHandler{sockets: &sync.Map{}, wg: &sync.WaitGroup{}}
		addr := "127.0.0.1:" + nextPort()
		app := NewServer(new(BuiltinEventHandler), &ServerOption{})

		app.OnRequest = func(netConn net.Conn, br *bufio.Reader, r *http.Request) {
			socket, err := app.GetUpgrader().UpgradeFromConn(netConn, br, r)
			if err != nil {
				return
			}
			handler.sockets.Store(socket, struct{}{})
			socket.ReadLoop()
		}
		go func() {
			if err := app.Run(addr); err != nil {
				as.NoError(err)
				return
			}
		}()

		time.Sleep(100 * time.Millisecond)

		count := 100
		for i := 0; i < count; i++ {
			client, _, err := NewClient(handler, &ClientOption{
				Addr: "ws://" + addr,
			})
			if err != nil {
				as.NoError(err)
				return
			}
			_ = client.WritePing(nil)
			go client.ReadLoop()
		}

		handler.wg.Add(count)
		b := NewBroadcaster(OpcodeText, internal.AlphabetNumeric.Generate(1000))
		handler.sockets.Range(func(key, value any) bool {
			_ = b.Broadcast(key.(*Conn))
			return true
		})
		b.Close()
		handler.wg.Wait()
	})

	t.Run("", func(t *testing.T) {
		handler := &broadcastHandler{sockets: &sync.Map{}, wg: &sync.WaitGroup{}}
		addr := "127.0.0.1:" + nextPort()
		app := NewServer(new(BuiltinEventHandler), &ServerOption{
			WriteMaxPayloadSize: 1000,
			Authorize: func(r *http.Request, session SessionStorage) bool {
				session.Store("name", 1)
				session.Store("name", 2)
				return true
			},
		})

		app.OnRequest = func(netConn net.Conn, br *bufio.Reader, r *http.Request) {
			socket, err := app.GetUpgrader().UpgradeFromConn(netConn, br, r)
			if err != nil {
				return
			}
			name, _ := socket.Session().Load("name")
			as.Equal(2, name)
			handler.sockets.Store(socket, struct{}{})
			socket.ReadLoop()
		}

		go func() {
			if err := app.Run(addr); err != nil {
				as.NoError(err)
				return
			}
		}()

		time.Sleep(100 * time.Millisecond)

		count := 100
		for i := 0; i < count; i++ {
			client, _, err := NewClient(handler, &ClientOption{
				Addr: "ws://" + addr,
			})
			if err != nil {
				as.NoError(err)
				return
			}
			go client.ReadLoop()
		}

		b := NewBroadcaster(OpcodeText, testdata)
		handler.sockets.Range(func(key, value any) bool {
			if err := b.Broadcast(key.(*Conn)); err == nil {
				handler.wg.Add(1)
			}
			return true
		})
		time.Sleep(500 * time.Millisecond)
		b.Close()
		handler.wg.Wait()
	})

	t.Run("conn closed", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		serverOption := &ServerOption{}
		clientOption := &ClientOption{}
		wg := &sync.WaitGroup{}
		wg.Add(1)

		serverHandler.onClose = func(socket *Conn, err error) {
			as.Error(err)
			wg.Done()
		}
		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		server.WriteClose(0, nil)
		broadcaster := NewBroadcaster(OpcodeText, internal.AlphabetNumeric.Generate(16))
		_ = broadcaster.Broadcast(server)
		wg.Wait()
	})
}

type broadcastHandler struct {
	BuiltinEventHandler
	wg      *sync.WaitGroup
	sockets *sync.Map
}

func (b broadcastHandler) OnMessage(socket *Conn, message *Message) {
	defer message.Close()
	b.wg.Done()
}

func TestRecovery(t *testing.T) {
	as := assert.New(t)
	serverHandler := new(webSocketMocker)
	clientHandler := new(webSocketMocker)
	serverOption := &ServerOption{Recovery: Recovery}
	clientOption := &ClientOption{}
	serverHandler.onMessage = func(socket *Conn, message *Message) {
		var m map[string]uint8
		m[""] = 1
	}
	server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
	go server.ReadLoop()
	go client.ReadLoop()
	as.NoError(client.WriteString("hi"))
	time.Sleep(100 * time.Millisecond)
}

func TestConn_Writev(t *testing.T) {
	t.Run("", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		serverOption := &ServerOption{}
		clientOption := &ClientOption{}
		wg := &sync.WaitGroup{}
		wg.Add(1)

		serverHandler.onMessage = func(socket *Conn, message *Message) {
			if bytes.Equal(message.Bytes(), []byte("hello, world!")) {
				wg.Done()
			}
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		err := client.Writev(OpcodeText, [][]byte{
			[]byte("he"),
			[]byte("llo"),
			[]byte(", world!"),
		}...)
		assert.NoError(t, err)
		wg.Wait()
	})

	t.Run("", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		serverOption := &ServerOption{}
		clientOption := &ClientOption{}
		wg := &sync.WaitGroup{}
		wg.Add(1)

		serverHandler.onMessage = func(socket *Conn, message *Message) {
			if bytes.Equal(message.Bytes(), []byte("hello, world!")) {
				wg.Done()
			}
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		client.WritevAsync(OpcodeText, [][]byte{
			[]byte("he"),
			[]byte("llo"),
			[]byte(", world!"),
		}, func(err error) {
			assert.NoError(t, err)
		})
		wg.Wait()
	})

	t.Run("", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		serverOption := &ServerOption{}
		clientOption := &ClientOption{}
		wg := &sync.WaitGroup{}
		wg.Add(1)

		serverHandler.onMessage = func(socket *Conn, message *Message) {
			if bytes.Equal(message.Bytes(), []byte("hello, world!")) {
				wg.Done()
			}
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		err := client.Writev(OpcodeText, [][]byte{
			[]byte("he"),
			[]byte("llo"),
			[]byte(", world!"),
		}...)
		assert.NoError(t, err)
		wg.Wait()
	})

	t.Run("", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		serverOption := &ServerOption{}
		clientOption := &ClientOption{
			CheckUtf8Enabled: true,
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		err := client.Writev(OpcodeText, [][]byte{
			[]byte("山高月小"),
			[]byte("水落石出")[2:],
		}...)
		assert.Error(t, err)
	})
}

func TestConn_Async(t *testing.T) {
	conn := &Conn{writeQueue: workerQueue{maxConcurrency: 1}}
	wg := sync.WaitGroup{}
	wg.Add(100)
	var arr1, arr2 []int64
	mu := &sync.Mutex{}
	for i := 1; i <= 100; i++ {
		x := int64(i)
		arr1 = append(arr1, x)
		conn.Async(func() {
			mu.Lock()
			arr2 = append(arr2, x)
			mu.Unlock()
			wg.Done()
		})
	}
	wg.Wait()
	assert.True(t, internal.IsSameSlice(arr1, arr2))
}
