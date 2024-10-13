package internal

import (
	"bytes"
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIOUtil(t *testing.T) {
	as := assert.New(t)

	t.Run("", func(t *testing.T) {
		reader := strings.NewReader("hello")
		p := make([]byte, 5)
		err := ReadN(reader, p)
		as.Nil(err)
	})

	t.Run("", func(t *testing.T) {
		writer := bytes.NewBufferString("")
		err := WriteN(writer, nil)
		as.NoError(err)
	})

	t.Run("", func(t *testing.T) {
		writer := bytes.NewBufferString("")
		p := []byte("hello")
		err := WriteN(writer, p)
		as.NoError(err)
	})
}

func TestBuffers_WriteTo(t *testing.T) {
	t.Run("", func(t *testing.T) {
		b := Buffers{
			[]byte("he"),
			[]byte("llo"),
		}
		w := bytes.NewBufferString("")
		b.WriteTo(w)
		n, _ := b.WriteTo(w)
		assert.Equal(t, w.String(), "hellohello")
		assert.Equal(t, n, int64(5))
		assert.Equal(t, b.Len(), 5)
		assert.True(t, b.CheckEncoding(true, 1))
	})

	t.Run("", func(t *testing.T) {
		conn, _ := net.Pipe()
		_ = conn.Close()
		b := Buffers{
			[]byte("he"),
			[]byte("llo"),
		}
		_, err := b.WriteTo(conn)
		assert.Error(t, err)
	})

	t.Run("", func(t *testing.T) {
		str := "你好"
		b := Buffers{
			[]byte("he"),
			[]byte(str[2:]),
		}
		assert.False(t, b.CheckEncoding(true, 1))
	})
}

func TestBytes_WriteTo(t *testing.T) {
	t.Run("", func(t *testing.T) {
		b := Bytes("hello")
		w := bytes.NewBufferString("")
		b.WriteTo(w)
		n, _ := b.WriteTo(w)
		assert.Equal(t, w.String(), "hellohello")
		assert.Equal(t, n, int64(5))
		assert.Equal(t, b.Len(), 5)
	})

	t.Run("", func(t *testing.T) {
		str := "你好"
		b := Bytes(str[2:])
		assert.False(t, b.CheckEncoding(true, 1))
		assert.True(t, b.CheckEncoding(false, 1))
		assert.True(t, b.CheckEncoding(true, 2))
	})
}
