package gws

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func validateServerOption(as *assert.Assertions, u *Upgrader) {
	option := u.option
	config := u.option.getConfig()
	as.Equal(config.ParallelEnabled, option.ParallelEnabled)
	as.Equal(config.ParallelGolimit, option.ParallelGolimit)
	as.Equal(config.ReadMaxPayloadSize, option.ReadMaxPayloadSize)
	as.Equal(config.WriteMaxPayloadSize, option.WriteMaxPayloadSize)
	as.Equal(config.CheckUtf8Enabled, option.CheckUtf8Enabled)
	as.Equal(config.ReadBufferSize, option.ReadBufferSize)
	as.Equal(config.WriteBufferSize, option.WriteBufferSize)
	as.NotNil(config.brPool)
	as.NotNil(config.Recovery)
	as.Equal(config.Logger, defaultLogger)

	_, ok := u.option.NewSession().(*smap)
	as.True(ok)
}

func validateClientOption(as *assert.Assertions, option *ClientOption) {
	config := option.getConfig()
	as.Equal(config.ParallelEnabled, option.ParallelEnabled)
	as.Equal(config.ParallelGolimit, option.ParallelGolimit)
	as.Equal(config.ReadMaxPayloadSize, option.ReadMaxPayloadSize)
	as.Equal(config.WriteMaxPayloadSize, option.WriteMaxPayloadSize)
	as.Equal(config.CheckUtf8Enabled, option.CheckUtf8Enabled)
	as.Equal(config.ReadBufferSize, option.ReadBufferSize)
	as.Equal(config.WriteBufferSize, option.WriteBufferSize)
	as.Nil(config.brPool)
	as.NotNil(config.Recovery)
	as.Equal(config.Logger, defaultLogger)

	_, ok := option.NewSession().(*smap)
	as.True(ok)
}

// 检查默认配置
func TestDefaultUpgrader(t *testing.T) {
	as := assert.New(t)
	updrader := NewUpgrader(new(BuiltinEventHandler), &ServerOption{
		ResponseHeader: http.Header{
			"Sec-Websocket-Extensions": []string{"chat"},
			"X-Server":                 []string{"gws"},
		},
	})
	config := updrader.option.getConfig()
	as.Equal(false, config.ParallelEnabled)
	as.Equal(false, config.CheckUtf8Enabled)
	as.Equal(defaultParallelGolimit, config.ParallelGolimit)
	as.Equal(defaultReadMaxPayloadSize, config.ReadMaxPayloadSize)
	as.Equal(defaultWriteMaxPayloadSize, config.WriteMaxPayloadSize)
	as.Equal(defaultHandshakeTimeout, updrader.option.HandshakeTimeout)
	as.NotNil(updrader.eventHandler)
	as.NotNil(config)
	as.NotNil(updrader.option)
	as.NotNil(updrader.option.ResponseHeader)
	as.NotNil(updrader.option.Authorize)
	as.NotNil(updrader.option.NewSession)
	as.Nil(updrader.option.SubProtocols)
	as.Equal("", updrader.option.ResponseHeader.Get("Sec-Websocket-Extensions"))
	as.Equal("gws", updrader.option.ResponseHeader.Get("X-Server"))
	validateServerOption(as, updrader)
}

func TestReadServerOption(t *testing.T) {
	as := assert.New(t)
	updrader := NewUpgrader(new(BuiltinEventHandler), &ServerOption{
		ParallelEnabled:    true,
		ParallelGolimit:    4,
		ReadMaxPayloadSize: 1024,
		HandshakeTimeout:   10 * time.Second,
	})
	config := updrader.option.getConfig()
	as.Equal(true, config.ParallelEnabled)
	as.Equal(4, config.ParallelGolimit)
	as.Equal(1024, config.ReadMaxPayloadSize)
	as.Equal(10*time.Second, updrader.option.HandshakeTimeout)
	validateServerOption(as, updrader)
}

func TestDefaultClientOption(t *testing.T) {
	as := assert.New(t)
	option := &ClientOption{}
	NewClient(new(BuiltinEventHandler), option)

	config := option.getConfig()
	as.Nil(config.brPool)
	as.Equal(false, config.ParallelEnabled)
	as.Equal(false, config.CheckUtf8Enabled)
	as.Equal(defaultParallelGolimit, config.ParallelGolimit)
	as.Equal(defaultReadMaxPayloadSize, config.ReadMaxPayloadSize)
	as.Equal(defaultWriteMaxPayloadSize, config.WriteMaxPayloadSize)
	as.NotNil(config)
	as.Equal(0, len(option.RequestHeader))
	as.NotNil(option.NewSession)
	validateClientOption(as, option)
}

func TestNewSession(t *testing.T) {
	{
		option := &ServerOption{
			NewSession: func() SessionStorage { return NewConcurrentMap[string, any](16) },
		}
		initServerOption(option)
		_, ok := option.NewSession().(*ConcurrentMap[string, any])
		assert.True(t, ok)
	}

	{
		option := &ClientOption{
			NewSession: func() SessionStorage { return NewConcurrentMap[string, any](16) },
		}
		initClientOption(option)
		_, ok := option.NewSession().(*ConcurrentMap[string, any])
		assert.True(t, ok)
	}
}
