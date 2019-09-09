package ot

import "sync"

type cacheCategory struct {
	cat sync.Map
}

// Store saves category in the cache.
func (c *cacheCategory) Store(cat *Category) {
	c.cat.Store(cat.DisplayName, cat)
}

// Find returns the category by name.
func (c *cacheCategory) Find(name string) *Category {
	v, ok := c.cat.Load(name)
	if !ok {
		return nil
	}

	return v.(*Category).Copy()
}

// CacheCategory is a cache of the categories.
var CacheCategory = &cacheCategory{}
