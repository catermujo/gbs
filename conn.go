package gbs

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/catermujo/gbs/internal"
)

// Conn WebSocket连接
// WebSocket connection
type Conn struct {
	// ev Atomic value for storing errors
	ev        atomic.Value
	ss        SessionStorage
	conn      net.Conn
	handler   EventHandler
	readQueue channel
	config    *Config
	// br Buffered reader
	br                *bufio.Reader
	subprotocol       string
	continuationFrame continuationFrame
	writeQueue        workerQueue
	mu                sync.Mutex
	closed            uint32
	fh                frameHeader
	isServer          bool
}

func (c *Conn) UpdateHandler(handler EventHandler) {
	c.handler = handler
}

// ReadLoop
// 循环读取消息. 如果复用了HTTP Server, 建议开启goroutine, 阻塞会导致请求上下文无法被GC.
// Read messages in a loop.
// If HTTP Server is reused, it is recommended to enable goroutine, as blocking will prevent the context from being GC.
func (c *Conn) ReadLoop() {
	c.handler.OnOpen(c)

	// 无限循环读取消息, 如果发生错误则触发错误事件并退出循环
	// Infinite loop to read messages, if an error occurs, trigger the error event and exit the loop
	for {
		if err := c.readMessage(); err != nil {
			c.emitError(true, err)
			break
		}
	}

	err, ok := c.ev.Load().(error)
	c.handler.OnClose(c, internal.SelectValue(ok, err, errEmpty))

	// 回收资源
	// Reclaim resources
	if c.isServer {
		c.br.Reset(nil)
		c.config.brPool.Put(c.br)
		c.br = nil
	}
}

// 检查连接是否已关闭
// Checks if the connection is closed
func (c *Conn) IsClosed() bool {
	return atomic.LoadUint32(&c.closed) == 1
}

// 处理错误事件
// Handle the error event
func (c *Conn) emitError(reading bool, err error) {
	if err == nil {
		return
	}

	if atomic.CompareAndSwapUint32(&c.closed, 0, 1) {
		// 待发送的错误码和错误原因
		// Error code to be sent and cause of error
		sendCode, sendErr := internal.CloseGoingAway, error(internal.CloseGoingAway)
		if reading {
			switch v := err.(type) {
			case internal.StatusCode:
				sendCode, sendErr = v, v
			case *internal.Error:
				sendCode, sendErr, err = v.Code, v.Err, v.Err
			default:
				sendCode, sendErr = internal.CloseNormalClosure, err
			}
		}

		reason := append(sendCode.Bytes(), sendErr.Error()...)
		_ = c.writeClose(err, reason)
	}
}

// 处理关闭事件
// Handles the close event
func (c *Conn) emitClose(buf *bytes.Buffer) error {
	responseCode := internal.CloseNormalClosure
	realCode := internal.CloseNormalClosure.Uint16()
	switch buf.Len() {
	case 0:
		responseCode = 0
		realCode = 0
	case 1:
		responseCode = internal.CloseProtocolError
		realCode = uint16(buf.Bytes()[0])
		buf.Reset()
	default:
		var b [2]byte
		_, _ = buf.Read(b[0:])
		realCode = binary.BigEndian.Uint16(b[0:])
		switch realCode {
		case 1004, 1005, 1006, 1014, 1015:
			responseCode = internal.CloseProtocolError
		default:
			if realCode < 1000 || realCode >= 5000 || (realCode >= 1016 && realCode < 3000) {
				responseCode = internal.CloseProtocolError
			} else if realCode < 1016 {
				responseCode = internal.CloseNormalClosure
			} else {
				responseCode = internal.StatusCode(realCode)
			}
		}
		if !internal.CheckEncoding(c.config.CheckUtf8Enabled, uint8(OpcodeCloseConnection), buf.Bytes()) {
			responseCode = internal.CloseUnsupportedData
		}
	}
	if atomic.CompareAndSwapUint32(&c.closed, 0, 1) {
		_ = c.writeClose(&CloseError{Code: realCode, Reason: buf.Bytes()}, responseCode.Bytes())
	}
	return internal.CloseNormalClosure
}

// SetDeadline 设置连接的截止时间
// Sets the deadline for the connection
func (c *Conn) SetDeadline(t time.Time) error {
	err := c.conn.SetDeadline(t)
	c.emitError(false, err)
	return err
}

// SetReadDeadline 设置读取操作的截止时间
// Sets the deadline for read operations
func (c *Conn) SetReadDeadline(t time.Time) error {
	err := c.conn.SetReadDeadline(t)
	c.emitError(false, err)
	return err
}

// SetWriteDeadline 设置写入操作的截止时间
// Sets the deadline for write operations
func (c *Conn) SetWriteDeadline(t time.Time) error {
	err := c.conn.SetWriteDeadline(t)
	c.emitError(false, err)
	return err
}

// LocalAddr 返回本地网络地址
// Returns the local network address
func (c *Conn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// RemoteAddr 返回远程网络地址
// Returns the remote network address
func (c *Conn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// NetConn
// 获取底层的 TCP/TLS/KCP 等连接
// Gets the underlying TCP/TLS/KCP... connection
func (c *Conn) NetConn() net.Conn {
	return c.conn
}

// SetNoDelay 设置无延迟
// 控制操作系统是否应该延迟数据包传输以期望发送更少的数据包(Nagle算法).
// 默认值是 true（无延迟），这意味着数据在 Write 之后尽快发送.
// Controls whether the operating system should delay packet transmission in hopes of sending fewer packets (Nagle's algorithm).
// The default is true (no delay), meaning that data is sent as soon as possible after a Write.
func (c *Conn) SetNoDelay(noDelay bool) error {
	switch v := c.conn.(type) {
	case *net.TCPConn:
		return v.SetNoDelay(noDelay)

	case *tls.Conn:
		if netConn, ok := v.NetConn().(*net.TCPConn); ok {
			return netConn.SetNoDelay(noDelay)
		}
	}
	return nil
}

// SubProtocol 获取协商的子协议
// Gets the negotiated sub-protocol
func (c *Conn) SubProtocol() string { return c.subprotocol }

// Session 获取会话存储
// Gets the session storage
func (c *Conn) Session() SessionStorage { return c.ss }
