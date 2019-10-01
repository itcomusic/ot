package ot

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCacheCategory_Store(t *testing.T) {
	t.Parallel()

	cache := &cacheCategory{}
	c := &Category{DisplayName: "Name"}
	cache.Store(c)

	// change category in cache
	c.Data = make([]Value, 1)
	c.Data[0].Key = "Change"

	cat, ok := cache.cat.Load("Name")
	require.Equal(t, true, ok)
	assert.Equal(t, &Category{DisplayName: "Name"}, cat)
}

func TestCacheCategory_Find(t *testing.T) {
	t.Parallel()

	cache := &cacheCategory{}
	cache.Store(&Category{DisplayName: "Name", Data: []Value{}})
	cache.Store(&Category{DisplayName: "WithoutData"})

	for i, tt := range []struct {
		name string
		exp  *Category
	}{
		{name: "Name", exp: &Category{DisplayName: "Name", Data: []Value{}}},
		{name: "WithoutData", exp: &Category{DisplayName: "WithoutData"}},
		{name: "Nil", exp: nil},
	} {
		cat := cache.Find(tt.name)
		assert.Equal(t, tt.exp, cat, fmt.Sprintf("#%d", i))
	}
}
