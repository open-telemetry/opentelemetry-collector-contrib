// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package buffer

import (
	"context"
	"testing"
	"time"

	"github.com/opentelemetry/opentelemetry-log-collection/entry"
	"github.com/stretchr/testify/require"
)

func intEntry(i int) *entry.Entry {
	e := entry.New()
	e.Timestamp = time.Date(2006, 01, 02, 03, 04, 05, 06, time.UTC)
	e.Record = float64(i)
	return e
}

func writeN(t testing.TB, buffer Buffer, n, start int) {
	ctx := context.Background()
	for i := start; i < n+start; i++ {
		err := buffer.Add(ctx, intEntry(i))
		require.NoError(t, err)
	}
}

func readN(t testing.TB, buffer Buffer, n, start int) Clearer {
	entries := make([]*entry.Entry, n)
	f, readCount, err := buffer.Read(entries)
	require.NoError(t, err)
	require.Equal(t, n, readCount)
	for i := 0; i < n; i++ {
		require.Equal(t, intEntry(start+i), entries[i])
	}
	return f
}

func readWaitN(t testing.TB, buffer Buffer, n, start int) Clearer {
	entries := make([]*entry.Entry, n)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	f, readCount, err := buffer.ReadWait(ctx, entries)
	require.NoError(t, err)
	require.Equal(t, n, readCount)
	for i := 0; i < n; i++ {
		require.Equal(t, intEntry(start+i), entries[i])
	}
	return f
}

func flushN(t testing.TB, buffer Buffer, n, start int) {
	f := readN(t, buffer, n, start)
	f.MarkAllAsFlushed()
}

func panicOnErr(err error) {
	if err != nil {
		panic(err)
	}
}
