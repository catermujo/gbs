package gws

import (
	"bufio"
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"github.com/lxzan/gws/internal"
)

const (
	// 默认的并行协程限制
	// Default parallel goroutine limit
	defaultParallelGolimit = 8

	// 默认的读取最大负载大小
	// Default maximum payload size for reading
	defaultReadMaxPayloadSize = 16 * 1024 * 1024

	// 默认的写入最大负载大小
	// Default maximum payload size for writing
	defaultWriteMaxPayloadSize = 16 * 1024 * 1024

	// 默认的读取缓冲区大小
	// Default read buffer size
	defaultReadBufferSize = 4 * 1024

	// 默认的写入缓冲区大小
	// Default write buffer size
	defaultWriteBufferSize = 4 * 1024

	// 默认的握手超时时间
	// Default handshake timeout
	defaultHandshakeTimeout = 5 * time.Second

	// 默认的拨号超时时间
	// Default dial timeout
	defaultDialTimeout = 5 * time.Second
)

type (
	Config struct {
		// Logging tools
		Logger Logger

		// Memory pool for bufio.Reader
		brPool *internal.Pool[*bufio.Reader]

		// Message callback (OnMessage) recovery program
		Recovery func(logger Logger)

		// Maximum read message content length
		ReadMaxPayloadSize int

		// Size of the read buffer
		ReadBufferSize int

		// Maximum length of written message content
		WriteMaxPayloadSize int

		// Deprecated: Size of the write buffer, v1.4.5 version of this parameter is deprecated
		WriteBufferSize int

		// Limit on the number of concurrent goroutines used for parallel message processing (single connection)
		ParallelGolimit int

		// Whether to check the text utf8 encoding, turn off the performance will be better
		CheckUtf8Enabled bool

		// Whether to enable parallel message processing
		ParallelEnabled bool
	}

	// ServerOption 服务端配置
	// Server configurations
	ServerOption struct {
		// Logger
		Logger Logger

		// Create session storage space for custom SessionStorage implementations
		NewSession func() SessionStorage

		// Authentication function for connection establishment requests
		Authorize func(r *http.Request, session SessionStorage) bool

		// Additional response headers (may not be supported by the client)
		// https://www.rfc-editor.org/rfc/rfc6455.html#section-1.3
		ResponseHeader http.Header

		// Configuration
		config *Config

		// TLS configuration
		TlsConfig *tls.Config

		// Recovery function
		Recovery func(logger Logger)

		// WebSocket sub-protocol, handshake failure disconnects the connection
		SubProtocols []string

		// Handshake timeout duration
		HandshakeTimeout time.Duration

		// Maximum payload size for writing
		WriteMaxPayloadSize int

		// Read buffer size
		ReadBufferSize int

		// Maximum payload size for reading
		ReadMaxPayloadSize int

		// Parallel goroutine limit
		ParallelGolimit int

		// Deprecated: Size of the write buffer, v1.4.5 version of this parameter is deprecated
		WriteBufferSize int

		// Whether UTF-8 check is enabled
		CheckUtf8Enabled bool

		// Whether parallel processing is enabled
		ParallelEnabled bool
	}
)

// 删除受保护的 WebSocket 头部字段
// Removes protected WebSocket header fields
func (c *ServerOption) deleteProtectedHeaders() {
	c.ResponseHeader.Del(internal.Upgrade.Key)
	c.ResponseHeader.Del(internal.Connection.Key)
	c.ResponseHeader.Del(internal.SecWebSocketAccept.Key)
	c.ResponseHeader.Del(internal.SecWebSocketExtensions.Key)
	c.ResponseHeader.Del(internal.SecWebSocketProtocol.Key)
}

// 初始化服务器配置
// Initialize server options
func initServerOption(c *ServerOption) *ServerOption {
	if c == nil {
		c = new(ServerOption)
	}
	if c.ReadMaxPayloadSize <= 0 {
		c.ReadMaxPayloadSize = defaultReadMaxPayloadSize
	}
	if c.ParallelGolimit <= 0 {
		c.ParallelGolimit = defaultParallelGolimit
	}
	if c.ReadBufferSize <= 0 {
		c.ReadBufferSize = defaultReadBufferSize
	}
	if c.WriteMaxPayloadSize <= 0 {
		c.WriteMaxPayloadSize = defaultWriteMaxPayloadSize
	}
	if c.WriteBufferSize <= 0 {
		c.WriteBufferSize = defaultWriteBufferSize
	}
	if c.Authorize == nil {
		c.Authorize = func(r *http.Request, session SessionStorage) bool { return true }
	}
	if c.NewSession == nil {
		c.NewSession = func() SessionStorage { return newSmap() }
	}
	if c.ResponseHeader == nil {
		c.ResponseHeader = http.Header{}
	}
	if c.HandshakeTimeout <= 0 {
		c.HandshakeTimeout = defaultHandshakeTimeout
	}
	if c.Logger == nil {
		c.Logger = defaultLogger
	}
	if c.Recovery == nil {
		c.Recovery = func(logger Logger) {}
	}

	c.deleteProtectedHeaders()

	c.config = &Config{
		ParallelEnabled:     c.ParallelEnabled,
		ParallelGolimit:     c.ParallelGolimit,
		ReadMaxPayloadSize:  c.ReadMaxPayloadSize,
		ReadBufferSize:      c.ReadBufferSize,
		WriteMaxPayloadSize: c.WriteMaxPayloadSize,
		WriteBufferSize:     c.WriteBufferSize,
		CheckUtf8Enabled:    c.CheckUtf8Enabled,
		Recovery:            c.Recovery,
		Logger:              c.Logger,
		brPool: internal.NewPool(func() *bufio.Reader {
			return bufio.NewReaderSize(nil, c.ReadBufferSize)
		}),
	}

	return c
}

// 获取服务器配置
// Get server configuration
func (c *ServerOption) getConfig() *Config { return c.config }

// ClientOption 客户端配置
// Client configurations
type ClientOption struct {
	Logger Logger

	// For custom SessionStorage implementations
	NewSession func() SessionStorage

	// The default is to return the net.Dialer instance.
	// Can also be used to set a proxy, for example:
	// NewDialer: func() (proxy.Dialer, error) {
	//     return proxy.SOCKS5("tcp", "127.0.0.1:1080", nil, nil)
	// },
	NewDialer func() (Dialer, error)

	// TLS configuration
	TlsConfig *tls.Config

	// Extra request headers
	RequestHeader http.Header

	// Recovery function
	Recovery func(logger Logger)

	// Server address, e.g., wss://example.com/connect
	Addr string

	// Maximum payload size for reading
	ReadMaxPayloadSize int

	// Maximum payload size for writing
	WriteMaxPayloadSize int

	// Read buffer size
	ReadBufferSize int

	// Handshake timeout duration
	HandshakeTimeout time.Duration

	// Parallel goroutine limit
	ParallelGolimit int

	// Deprecated: Size of the write buffer, v1.4.5 version of this parameter is deprecated
	WriteBufferSize int

	// Whether UTF-8 check is enabled
	CheckUtf8Enabled bool

	// Whether parallel processing is enabled
	ParallelEnabled bool
}

// 初始化客户端配置
// Initialize client options
func initClientOption(c *ClientOption) *ClientOption {
	if c == nil {
		c = new(ClientOption)
	}
	if c.ReadMaxPayloadSize <= 0 {
		c.ReadMaxPayloadSize = defaultReadMaxPayloadSize
	}
	if c.ParallelGolimit <= 0 {
		c.ParallelGolimit = defaultParallelGolimit
	}
	if c.ReadBufferSize <= 0 {
		c.ReadBufferSize = defaultReadBufferSize
	}
	if c.WriteMaxPayloadSize <= 0 {
		c.WriteMaxPayloadSize = defaultWriteMaxPayloadSize
	}
	if c.WriteBufferSize <= 0 {
		c.WriteBufferSize = defaultWriteBufferSize
	}
	if c.HandshakeTimeout <= 0 {
		c.HandshakeTimeout = defaultHandshakeTimeout
	}
	if c.RequestHeader == nil {
		c.RequestHeader = http.Header{}
	}
	if c.NewDialer == nil {
		c.NewDialer = func() (Dialer, error) { return &net.Dialer{Timeout: defaultDialTimeout}, nil }
	}
	if c.NewSession == nil {
		c.NewSession = func() SessionStorage { return newSmap() }
	}
	if c.Logger == nil {
		c.Logger = defaultLogger
	}
	if c.Recovery == nil {
		c.Recovery = func(logger Logger) {}
	}
	return c
}

// 将 ClientOption 的配置转换为 Config 并返回
// Converts the ClientOption configuration to Config and returns it
func (c *ClientOption) getConfig() *Config {
	config := &Config{
		ParallelEnabled:     c.ParallelEnabled,
		ParallelGolimit:     c.ParallelGolimit,
		ReadMaxPayloadSize:  c.ReadMaxPayloadSize,
		ReadBufferSize:      c.ReadBufferSize,
		WriteMaxPayloadSize: c.WriteMaxPayloadSize,
		WriteBufferSize:     c.WriteBufferSize,
		CheckUtf8Enabled:    c.CheckUtf8Enabled,
		Recovery:            c.Recovery,
		Logger:              c.Logger,
	}
	return config
}
