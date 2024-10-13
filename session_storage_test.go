package gws

import (
	"testing"

	"github.com/dolthub/maphash"
	"github.com/lxzan/gws/internal"
	"github.com/stretchr/testify/assert"
)

func TestMap(t *testing.T) {
	as := assert.New(t)
	m1 := make(map[string]any)
	m2 := newSmap()
	count := internal.AlphabetNumeric.Intn(1000)
	for i := 0; i < count; i++ {
		key := string(internal.AlphabetNumeric.Generate(10))
		val := internal.AlphabetNumeric.Uint32()
		m1[key] = val
		m2.Store(key, val)
	}

	keys := make([]string, 0)
	for k := range m1 {
		keys = append(keys, k)
	}
	for i := 0; i < len(keys)/2; i++ {
		delete(m1, keys[i])
		m2.Delete(keys[i])
	}

	for k, v := range m1 {
		v1, ok := m2.Load(k)
		as.Equal(true, ok)
		as.Equal(v, v1)
	}
	as.Equal(len(m1), m2.Len())
}

func TestSliceMap(t *testing.T) {
	as := assert.New(t)
	m := newSmap()
	m.Store("hong", 1)
	m.Store("mei", 2)
	m.Store("ming", 3)
	{
		v, _ := m.Load("hong")
		as.Equal(1, v)
	}
	{
		m.Delete("hong")
		v, ok := m.Load("hong")
		as.Equal(false, ok)
		as.Nil(v)

		m.Store("hong", 4)
		v, _ = m.Load("hong")
		as.Equal(4, v)
	}
}

func TestMap_Range(t *testing.T) {
	as := assert.New(t)
	m1 := make(map[any]any)
	m2 := newSmap()
	count := 1000
	for i := 0; i < count; i++ {
		key := string(internal.AlphabetNumeric.Generate(10))
		val := internal.AlphabetNumeric.Uint32()
		m1[key] = val
		m2.Store(key, val)
	}

	{
		var keys []any
		m2.Range(func(key string, value any) bool {
			v, ok := m1[key]
			as.Equal(true, ok)
			as.Equal(v, value)
			keys = append(keys, key)
			return len(keys) < 100
		})
		as.Equal(100, len(keys))
	}

	{
		var keys []any
		m2.Range(func(key string, value any) bool {
			v, ok := m1[key]
			as.Equal(true, ok)
			as.Equal(v, value)
			keys = append(keys, key)
			return true
		})
		as.Equal(1000, len(keys))
	}
}

func TestConcurrentMap(t *testing.T) {
	as := assert.New(t)
	m1 := make(map[string]any)
	m2 := NewConcurrentMap[string, uint32]()
	as.Equal(m2.num, uint64(16))
	count := internal.AlphabetNumeric.Intn(1000)
	for i := 0; i < count; i++ {
		key := string(internal.AlphabetNumeric.Generate(10))
		val := internal.AlphabetNumeric.Uint32()
		m1[key] = val
		m2.Store(key, val)
	}

	keys := make([]string, 0)
	for k := range m1 {
		keys = append(keys, k)
	}
	for i := 0; i < len(keys)/2; i++ {
		delete(m1, keys[i])
		m2.Delete(keys[i])
	}

	for k, v := range m1 {
		v1, ok := m2.Load(k)
		as.Equal(true, ok)
		as.Equal(v, v1)
	}
	as.Equal(len(m1), m2.Len())

	t.Run("", func(t *testing.T) {
		sum := 0
		cm := NewConcurrentMap[string, int](8, 8)
		for _, item := range cm.shardings {
			sum += len(item.m)
		}
		assert.Equal(t, sum, 0)
	})
}

func TestConcurrentMap_Range(t *testing.T) {
	as := assert.New(t)
	m1 := make(map[any]any)
	m2 := NewConcurrentMap[string, uint32](13)
	count := 1000
	for i := 0; i < count; i++ {
		key := string(internal.AlphabetNumeric.Generate(10))
		val := internal.AlphabetNumeric.Uint32()
		m1[key] = val
		m2.Store(key, val)
	}

	{
		var keys []any
		m2.Range(func(key string, value uint32) bool {
			v, ok := m1[key]
			as.Equal(true, ok)
			as.Equal(v, value)
			keys = append(keys, key)
			return len(keys) < 100
		})
		as.Equal(100, len(keys))
	}

	{
		var keys []any
		m2.Range(func(key string, value uint32) bool {
			v, ok := m1[key]
			as.Equal(true, ok)
			as.Equal(v, value)
			keys = append(keys, key)
			return true
		})
		as.Equal(1000, len(keys))
	}
}

func TestHash(t *testing.T) {
	h := maphash.NewHasher[string]()
	for i := 0; i < 1000; i++ {
		a := string(internal.AlphabetNumeric.Generate(16))
		b := string(internal.AlphabetNumeric.Generate(16))
		assert.Equal(t, h.Hash(a), h.Hash(a))
		assert.NotEqual(t, h.Hash(a), h.Hash(b))
	}
}
