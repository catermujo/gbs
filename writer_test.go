package gws

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

	"github.com/lxzan/gws/internal"
	"github.com/stretchr/testify/assert"
)

func testWrite(c *Conn, fin bool, opcode Opcode, payload []byte) error {
	useCompress := c.pd.Enabled && opcode.isDataFrame() && len(payload) >= c.pd.Threshold
	if useCompress {
		buf := bytes.NewBufferString("")
		err := c.deflater.Compress(internal.Bytes(payload), buf, c.cpsWindow.dict)
		if err != nil {
			return internal.NewError(internal.CloseInternalErr, err)
		}
		payload = buf.Bytes()
	}
	if len(payload) > c.config.WriteMaxPayloadSize {
		return internal.CloseMessageTooLarge
	}

	header := frameHeader{}
	n := len(payload)
	headerLength, maskBytes := header.GenerateHeader(c.isServer, fin, useCompress, opcode, n)
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

func TestWriteBigMessage(t *testing.T) {
	t.Run("", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		serverOption := &ServerOption{WriteMaxPayloadSize: 16}
		clientOption := &ClientOption{}
		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()
		err := server.WriteMessage(OpcodeText, internal.AlphabetNumeric.Generate(128))
		assert.Error(t, err)
	})

	t.Run("", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		serverOption := &ServerOption{
			WriteMaxPayloadSize: 16,
			PermessageDeflate:   PermessageDeflate{Enabled: true, Threshold: 1},
		}
		clientOption := &ClientOption{
			PermessageDeflate: PermessageDeflate{Enabled: true},
		}
		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()
		err := server.WriteMessage(OpcodeText, internal.AlphabetNumeric.Generate(128))
		assert.Error(t, err)
	})

	t.Run("", func(t *testing.T) {
		wg := &sync.WaitGroup{}
		wg.Add(1)
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		serverHandler.onClose = func(socket *Conn, err error) {
			assert.True(t, errors.Is(err, internal.CloseMessageTooLarge))
			wg.Done()
		}
		serverOption := &ServerOption{
			ReadMaxPayloadSize: 128,
			PermessageDeflate:  PermessageDeflate{Enabled: true, Threshold: 1},
		}
		clientOption := &ClientOption{
			ReadMaxPayloadSize: 128 * 1024,
			PermessageDeflate:  PermessageDeflate{Enabled: true, Threshold: 1},
		}
		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		buf := bytes.NewBufferString("")
		for i := 0; i < 64*1024; i++ {
			buf.WriteString("a")
		}
		err := client.WriteMessage(OpcodeText, buf.Bytes())
		assert.NoError(t, err)
		wg.Wait()
	})
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
		pd := PermessageDeflate{
			Enabled: true,
		}
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		serverOption := &ServerOption{
			PermessageDeflate: pd,
		}
		clientOption := &ClientOption{
			PermessageDeflate: pd,
		}
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
		pd := PermessageDeflate{
			Enabled: true,
		}
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		serverOption := &ServerOption{
			PermessageDeflate: pd,
		}
		clientOption := &ClientOption{
			PermessageDeflate: pd,
		}
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
		app := NewServer(new(BuiltinEventHandler), &ServerOption{
			PermessageDeflate: PermessageDeflate{Enabled: true},
		})

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
			compress := i%2 == 0
			client, _, err := NewClient(handler, &ClientOption{
				Addr:              "ws://" + addr,
				PermessageDeflate: PermessageDeflate{Enabled: compress},
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
			PermessageDeflate:   PermessageDeflate{Enabled: true},
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
			compress := i%2 == 0
			client, _, err := NewClient(handler, &ClientOption{
				Addr:              "ws://" + addr,
				PermessageDeflate: PermessageDeflate{Enabled: compress},
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
		serverOption := &ServerOption{
			PermessageDeflate: PermessageDeflate{
				Enabled:               true,
				ServerContextTakeover: true,
				ClientContextTakeover: true,
				Threshold:             1,
			},
		}
		clientOption := &ClientOption{
			PermessageDeflate: PermessageDeflate{
				Enabled:               true,
				ServerContextTakeover: true,
				ClientContextTakeover: true,
				Threshold:             1,
			},
		}
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
		serverOption := &ServerOption{
			PermessageDeflate: PermessageDeflate{
				Enabled:               true,
				ServerContextTakeover: true,
				ClientContextTakeover: true,
				Threshold:             1,
			},
		}
		clientOption := &ClientOption{
			CheckUtf8Enabled: true,
			PermessageDeflate: PermessageDeflate{
				Enabled:               true,
				ServerContextTakeover: true,
				ClientContextTakeover: true,
				Threshold:             1,
			},
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

func TestConn_WriteFile(t *testing.T) {
	t.Run("context_take_over 1", func(t *testing.T) {
		pd := PermessageDeflate{
			Enabled:               true,
			ServerContextTakeover: true,
			ClientContextTakeover: true,
			Threshold:             1,
		}
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		serverOption := &ServerOption{
			PermessageDeflate: pd,
		}
		clientOption := &ClientOption{
			PermessageDeflate: pd,
		}
		wg := &sync.WaitGroup{}
		wg.Add(1)

		content := internal.AlphabetNumeric.Generate(512 * 1024)
		clientHandler.onMessage = func(socket *Conn, message *Message) {
			if bytes.Equal(message.Bytes(), content) {
				wg.Done()
			}
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		err := server.WriteFile(OpcodeBinary, bytes.NewReader(content))
		assert.NoError(t, err)
		wg.Wait()
	})

	t.Run("context_take_over 2", func(t *testing.T) {
		pd := PermessageDeflate{
			Enabled:               true,
			ServerContextTakeover: true,
			ClientContextTakeover: true,
			ServerMaxWindowBits:   15,
			ClientMaxWindowBits:   15,
			Threshold:             1,
		}
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		serverOption := &ServerOption{
			PermessageDeflate: pd,
		}
		clientOption := &ClientOption{
			PermessageDeflate: pd,
		}
		wg := &sync.WaitGroup{}
		wg.Add(1)

		content := internal.AlphabetNumeric.Generate(512 * 1024)
		clientHandler.onMessage = func(socket *Conn, message *Message) {
			if bytes.Equal(message.Bytes(), content) {
				wg.Done()
			}
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		err := server.WriteFile(OpcodeBinary, bytes.NewReader(content))
		assert.NoError(t, err)
		wg.Wait()
	})

	t.Run("context_take_over 3", func(t *testing.T) {
		pd := PermessageDeflate{
			Enabled:               true,
			ServerContextTakeover: true,
			ClientContextTakeover: true,
			ServerMaxWindowBits:   15,
			ClientMaxWindowBits:   15,
			Threshold:             1,
		}
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		serverOption := &ServerOption{
			PermessageDeflate: pd,
		}
		clientOption := &ClientOption{
			PermessageDeflate: pd,
		}
		count := 1000
		wg := &sync.WaitGroup{}
		wg.Add(count)

		clientHandler.onMessage = func(socket *Conn, message *Message) {
			wg.Done()
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		for i := 0; i < count; i++ {
			length := 128*1024 + internal.AlphabetNumeric.Intn(10)
			content := internal.AlphabetNumeric.Generate(length)
			err := server.WriteFile(OpcodeBinary, bytes.NewReader(content))
			assert.NoError(t, err)
		}
		wg.Wait()
	})

	t.Run("no_context_take_over", func(t *testing.T) {
		pd := PermessageDeflate{
			Enabled:               true,
			ServerContextTakeover: false,
			ClientContextTakeover: false,
			Threshold:             1,
		}
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		serverOption := &ServerOption{
			PermessageDeflate: pd,
		}
		clientOption := &ClientOption{
			PermessageDeflate: pd,
		}
		wg := &sync.WaitGroup{}
		wg.Add(1)

		content := internal.AlphabetNumeric.Generate(512 * 1024)
		serverHandler.onMessage = func(socket *Conn, message *Message) {
			if bytes.Equal(message.Bytes(), content) {
				wg.Done()
			}
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		err := client.WriteFile(OpcodeBinary, bytes.NewReader(content))
		assert.NoError(t, err)
		wg.Wait()
	})

	t.Run("no_compress", func(t *testing.T) {
		pd := PermessageDeflate{
			Enabled: false,
		}
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		serverOption := &ServerOption{
			PermessageDeflate: pd,
		}
		clientOption := &ClientOption{
			PermessageDeflate: pd,
		}
		wg := &sync.WaitGroup{}
		wg.Add(1)

		content := internal.AlphabetNumeric.Generate(512 * 1024)
		serverHandler.onMessage = func(socket *Conn, message *Message) {
			if bytes.Equal(message.Bytes(), content) {
				wg.Done()
			}
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		err := client.WriteFile(OpcodeBinary, bytes.NewReader(content))
		assert.NoError(t, err)
		wg.Wait()
	})

	t.Run("close 1", func(t *testing.T) {
		pd := PermessageDeflate{
			Enabled: false,
		}
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		serverOption := &ServerOption{
			PermessageDeflate: pd,
		}
		clientOption := &ClientOption{
			PermessageDeflate: pd,
		}
		wg := &sync.WaitGroup{}
		wg.Add(1)

		content := internal.AlphabetNumeric.Generate(512 * 1024)
		serverHandler.onClose = func(socket *Conn, err error) {
			if ev, ok := err.(*CloseError); ok && ev.Code == 1000 {
				wg.Done()
			}
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		client.WriteClose(1000, nil)
		err := client.WriteFile(OpcodeBinary, bytes.NewReader(content))
		assert.Error(t, err)
		wg.Wait()
	})

	t.Run("msg too big", func(t *testing.T) {
		pd := PermessageDeflate{
			Enabled: false,
		}
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		serverOption := &ServerOption{
			PermessageDeflate: pd,
		}
		clientOption := &ClientOption{
			PermessageDeflate:   pd,
			WriteMaxPayloadSize: 1024,
		}
		wg := &sync.WaitGroup{}
		wg.Add(1)

		content := internal.AlphabetNumeric.Generate(512 * 1024)
		clientHandler.onClose = func(socket *Conn, err error) {
			wg.Done()
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		err := client.WriteFile(OpcodeBinary, bytes.NewReader(content))
		assert.Error(t, err)
		wg.Wait()
	})

	t.Run("", func(t *testing.T) {
		deflater := newBigDeflater(true, PermessageDeflate{
			Enabled:             true,
			ServerMaxWindowBits: 12,
			ClientMaxWindowBits: 12,
		})
		fw := &flateWriter{cb: func(index int, eof bool, p []byte) error {
			return nil
		}}
		reader := &readerWrapper{r: new(writerTo), sw: new(slideWindow)}
		err := deflater.Compress(reader, fw, nil)
		assert.Error(t, err)
	})

	t.Run("", func(t *testing.T) {
		deflater := newBigDeflater(true, PermessageDeflate{
			Enabled:             true,
			ServerMaxWindowBits: 12,
			ClientMaxWindowBits: 12,
		})
		fw := &flateWriter{cb: func(index int, eof bool, p []byte) error {
			return errors.New("2")
		}}
		reader := &readerWrapper{r: new(writerTo), sw: new(slideWindow)}
		err := deflater.Compress(reader, fw, nil)
		assert.Error(t, err)
	})

	t.Run("", func(t *testing.T) {
		fw := &flateWriter{
			cb: func(index int, eof bool, p []byte) error {
				return nil
			},
			buffers: []*bytes.Buffer{
				bytes.NewBufferString("he"),
				bytes.NewBufferString("llo"),
			},
		}
		err := fw.Flush()
		assert.NoError(t, err)
	})
}
