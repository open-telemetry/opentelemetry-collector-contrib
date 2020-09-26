// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mapwithexpiry

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMapWithExpiry_add(t *testing.T) {
	store := NewMapWithExpiry(time.Second)
	store.Set("key1", "value1")
	val, ok := store.Get("key1")
	assert.Equal(t, true, ok)
	assert.Equal(t, "value1", val.(string))

	val, ok = store.Get("key2")
	assert.Equal(t, false, ok)
	assert.Equal(t, nil, val)
}

func TestMapWithExpiry_cleanup(t *testing.T) {
	store := NewMapWithExpiry(time.Second)
	store.Set("key1", "value1")

	store.CleanUp(time.Now())
	val, ok := store.Get("key1")
	assert.Equal(t, true, ok)
	assert.Equal(t, "value1", val.(string))
	assert.Equal(t, 1, store.Size())

	time.Sleep(time.Second)
	store.CleanUp(time.Now())
	val, ok = store.Get("key1")
	assert.Equal(t, false, ok)
	assert.Equal(t, nil, val)
	assert.Equal(t, 0, store.Size())
}

func TestMapWithExpiry_concurrency(t *testing.T) {
	store := NewMapWithExpiry(time.Second)
	store.Set("sum", 0)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		for i := 0; i < 30; i++ {
			store.Lock()
			sum, _ := store.Get("sum")
			newSum := sum.(int) + 1
			store.Set("sum", newSum)
			store.Unlock()
		}
		wg.Done()
	}()

	go func() {
		for i := 0; i < 30; i++ {
			store.Lock()
			sum, _ := store.Get("sum")
			newSum := sum.(int) - 1
			store.Set("sum", newSum)
			store.Unlock()
		}
		wg.Done()
	}()
	wg.Wait()
	sum, _ := store.Get("sum")
	assert.Equal(t, 0, sum)
}
