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

// runOnceEntry guards a single keyed execution so that concurrent first-callers
// run the function exactly once and all observe the same cached result.
type runOnceEntry struct {
	once sync.Once
	err  error
}

// RunOnce ensures that a function identified by the
// given key is only executed the first time. Subequent calls
// will return a cached response.
//
// RunOnce is safe for concurrent use: if multiple goroutines call it with the
// same key simultaneously, exactly one executes f while the others block until
// it completes, then all return the cached result. This guarantee matters for
// shared-setup keys (e.g. writing the tfvars file or generating the kubeconfig)
// that are invoked from parallel stage operations.
func RunOnce(key string, f func() error) error {
	v, _ := runOnceStore.LoadOrStore(key, &runOnceEntry{})
	e := v.(*runOnceEntry)
	e.once.Do(func() {
		e.err = f()
	})
	return e.err
}
