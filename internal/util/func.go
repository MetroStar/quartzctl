package util

import (
	"sync"
)

var (
	runOnceStore sync.Map
)

// RunOnce ensures that a function identified by the
// given key is only executed the first time. Subequent calls
// will return a cached response.
func RunOnce(key string, f func() error) error {
	if v, loaded := runOnceStore.Load(key); loaded {
		if v != nil {
			return v.(error)
		}
		return nil
	}

	err := f()
	runOnceStore.Store(key, err)
	return err
}
