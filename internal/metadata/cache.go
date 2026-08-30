package metadata

import "sync"

// DefaultCacheTTLMs is the max age of a local epoch check before re-reading DB.
const DefaultCacheTTLMs = 2000

// Cache is the in-process metadata cache (epoch-checked against Postgres).
type Cache struct {
	mu               sync.RWMutex
	objects          map[string]ObjectDefinition
	fields           map[string][]FieldDefinition
	describes        map[string]DescribeObject
	epoch            int64
	lastEpochCheckAt int64 // unix ms
	ttlMs            int64
}

// NewCache constructs an empty metadata cache.
func NewCache(ttlMs int64) *Cache {
	if ttlMs <= 0 {
		ttlMs = DefaultCacheTTLMs
	}
	return &Cache{
		objects:   map[string]ObjectDefinition{},
		fields:    map[string][]FieldDefinition{},
		describes: map[string]DescribeObject{},
		ttlMs:     ttlMs,
	}
}

func (c *Cache) getEpoch() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.epoch
}

func (c *Cache) setEpoch(epoch, nowMs int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.epoch = epoch
	c.lastEpochCheckAt = nowMs
}

func (c *Cache) isEpochCheckFresh(nowMs int64) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.epoch > 0 && nowMs-c.lastEpochCheckAt < c.ttlMs
}

func (c *Cache) invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.objects = map[string]ObjectDefinition{}
	c.fields = map[string][]FieldDefinition{}
	c.describes = map[string]DescribeObject{}
}

func (c *Cache) getObject(apiName string) (ObjectDefinition, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	o, ok := c.objects[apiName]
	return o, ok
}

func (c *Cache) setObject(obj ObjectDefinition) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.objects[obj.APIName] = obj
}

func (c *Cache) setObjects(objs []ObjectDefinition) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.objects = map[string]ObjectDefinition{}
	for _, o := range objs {
		c.objects[o.APIName] = o
	}
}

func (c *Cache) getFields(objectAPIName string) ([]FieldDefinition, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	f, ok := c.fields[objectAPIName]
	if !ok {
		return nil, false
	}
	out := make([]FieldDefinition, len(f))
	copy(out, f)
	return out, true
}

func (c *Cache) setFields(objectAPIName string, fields []FieldDefinition) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.fields[objectAPIName] = fields
}

func (c *Cache) getDescribe(apiName string) (DescribeObject, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	d, ok := c.describes[apiName]
	return d, ok
}

func (c *Cache) setDescribe(desc DescribeObject) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.describes[desc.APIName] = desc
}

// ExpireEpochCheck forces the next read to re-check the DB epoch (tests).
func (c *Cache) ExpireEpochCheck() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastEpochCheckAt = 0
}
