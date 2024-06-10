package cache

import (
	"sync"
	"time"
)

type TimedCache[T comparable] struct {
	cache          map[T]time.Time
	timeout        time.Duration
	sweepThreshold int
	cleaningLock   *sync.RWMutex
}

func NewTimedCache[T comparable](timeout time.Duration, sweepThreshold int) *TimedCache[T] {
	return &TimedCache[T]{
		cache:          map[T]time.Time{},
		timeout:        timeout,
		sweepThreshold: sweepThreshold,
		cleaningLock:   &sync.RWMutex{},
	}
}

func (c *TimedCache[T]) Insert(k T) {
	c.cleaningLock.Lock()
	defer c.cleaningLock.Unlock()

	if len(c.cache) >= c.sweepThreshold {
		c.clean()
	}

	c.cache[k] = time.Now()
}

func (c *TimedCache[T]) GetAndRemove(k T) (time.Time, bool) {
	c.cleaningLock.RLock()
	v, ok := c.cache[k]
	c.cleaningLock.RUnlock()

	if ok {
		c.cleaningLock.Lock()
		delete(c.cache, k)
		c.cleaningLock.Unlock()
		return v, true
	}
	return time.Time{}, false
}

func (c *TimedCache[T]) clean() {
	sweepTime := time.Now()
	for k, v := range c.cache {
		if sweepTime.Sub(v) < c.timeout {
			delete(c.cache, k)
		}
	}
}
