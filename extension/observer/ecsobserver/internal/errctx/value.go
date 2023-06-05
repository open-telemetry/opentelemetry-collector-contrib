// Copyright The OpenTelemetry Authors
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

package errctx // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecsobserver/internal/errctx"

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrorWithValue indicates the error has some (could be just one) key value pairs
// attached to it during error wrapping.
type ErrorWithValue interface {
	error
	// Value returns a value attached to the error by key.
	// If the key does not exists, it returns nil, false.
	// The value saved can be nil, so it can also returns nil, true
	// to indicates a key exists but its value is nil.
	//
	// It does NOT do recursive Value calls (like context.Context).
	// For getting value from entire error chain, use ValueFrom.
	Value(key string) (v interface{}, ok bool)
}

// WithValue attaches a single key value pair to a non nil error.
// If err is nil, it does nothing and return nil.
// key has to be a non empty string otherwise it will panic.
// val can be nil, and the existence of a key with nil value
// can be distinguished using the bool return value in Value/ValueFrom call.
//
// It is a good practice to define the key as a constant strange instead of inline literal.
//
//		const taskErrKey = "task"
//		return errctx.WithValue(taskErrKey, myTask)
//	 task, ok := errctx.ValueFrom(err, taskErrKey)
func WithValue(err error, key string, val interface{}) error {
	if err == nil {
		return nil
	}
	// panic because this should not happen and there is no good way to return error when dealing w/ error.
	// This is also how context.WithValue is implemented.
	if key == "" {
		panic("empty key in WithValue")
	}
	// NOTE: we don't check if the value is nil because unlike context.Context
	// our value methods return a bool to indicates if the key exists or not
	// so we can allow user to save key with nil value, user's error inspection logic
	// need to be aware of that.

	return &valueError{
		key:   key,
		val:   val,
		inner: err,
	}
}

// WithValues attaches multiple key value pairs. The behavior is similar to WithValue.
func WithValues(err error, kvs map[string]interface{}) error {
	if err == nil {
		return nil
	}
	// make a shallow copy, and hope the values in map are not map ...
	m := make(map[string]interface{})
	for k, v := range kvs {
		if k == "" {
			panic("empty key in WithValues")
		}
		m[k] = v
	}
	return &valuesError{
		values: m,
		inner:  err,
	}
}

// ValueFrom traverse entire error chain and returns the value
// from the first ErrorWithValue that contains the key.
// e.g. for an error created using errctx.WithValue(errctx.WithValue(base, "k", "v1"), "k", "v2")
// ValueFrom(err, "k") returns "v2".
func ValueFrom(err error, key string) (interface{}, bool) {
	if err == nil {
		return nil, false
	}
	var verr ErrorWithValue
	if errors.As(err, &verr) {
		v, vok := verr.Value(key)
		if vok {
			return v, vok
		}
	}

	// Check if this is a wrapped error
	// I guess tail recursion should be optimized by compiler so we don't need to unroll it into for loop.
	return ValueFrom(errors.Unwrap(err), key)
}

// valueError only contains one pair, which is common
type valueError struct {
	key   string
	val   interface{}
	inner error
}

func (e *valueError) Error() string {
	return fmt.Sprintf("%s %s=%v", e.inner.Error(), e.key, e.val)
}

func (e *valueError) Value(key string) (interface{}, bool) {
	if key == e.key {
		return e.val, true
	}
	return nil, false
}

func (e *valueError) Unwrap() error {
	return e.inner
}

// valuesError contains multiple pairs
type valuesError struct {
	values map[string]interface{}
	inner  error
}

func (e *valuesError) Error() string {
	// NOTE: in order to have a consistent output, we sort the keys
	var keys []string
	for k := range e.values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	sb.WriteString(e.inner.Error())
	for _, k := range keys {
		v := e.values[k]
		sb.WriteString(fmt.Sprintf(" %s=%v", k, v))
	}
	return sb.String()
}

func (e *valuesError) Value(key string) (interface{}, bool) {
	v, ok := e.values[key]
	return v, ok
}

func (e *valuesError) Unwrap() error {
	return e.inner
}
