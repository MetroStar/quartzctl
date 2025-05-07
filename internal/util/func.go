// Copyright 2025 Metrostar Systems, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
