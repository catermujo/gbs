package internal

import (
	"io"
	"testing"
)

func TestBytesBuffer_Write(t *testing.T) {
	for i := 0; i < 1000; i++ {
		var s1 = AlphabetNumeric.Generate(R.Intn(1024))
		var s2 = AlphabetNumeric.Generate(R.Intn(512))
		var s3 = AlphabetNumeric.Generate(R.Intn(256))
		var buf = NewBytesBuffer(nil)
		buf.Write(s1)
		buf.Write(s2)
		buf.Write(s3)
		if string(s1)+string(s2)+string(s3) != string(buf.Bytes()) {
			t.Fail()
		}
	}
}

func TestBytesBuffer_Read(t *testing.T) {
	for i := 0; i < 1000; i++ {
		var buf = NewBytesBuffer(nil)
		var s1 = AlphabetNumeric.Generate(R.Intn(4 * 1024))
		buf.Write(s1)
		s2, err := io.ReadAll(buf)
		if err != nil {
			t.Fail()
			return
		}
		if string(s1) != string(s2) {
			t.Fail()
			return
		}
	}
}
