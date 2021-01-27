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

package elastic

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/opentelemetry/opentelemetry-log-collection/entry"
	"github.com/opentelemetry/opentelemetry-log-collection/testutil"
	"github.com/stretchr/testify/require"
)

func TestFindIndex(t *testing.T) {
	indexField := entry.NewRecordField("bar")
	output := &ElasticOutput{
		indexField: &indexField,
	}

	t.Run("StringValue", func(t *testing.T) {
		entry := entry.New()
		entry.Set(indexField, "testval")
		idx, err := output.FindIndex(entry)
		require.NoError(t, err)
		require.Equal(t, "testval", idx)
	})

	t.Run("ByteValue", func(t *testing.T) {
		entry := entry.New()
		entry.Set(indexField, []byte("testval"))
		idx, err := output.FindIndex(entry)
		require.NoError(t, err)
		require.Equal(t, "testval", idx)
	})

	t.Run("NoValue", func(t *testing.T) {
		entry := entry.New()
		_, err := output.FindIndex(entry)
		require.Error(t, err)
	})

	t.Run("IndexFieldUnset", func(t *testing.T) {
		entry := entry.New()
		output := &ElasticOutput{}
		idx, err := output.FindIndex(entry)
		require.NoError(t, err)
		require.Equal(t, "default", idx)
	})
}

func TestFindID(t *testing.T) {
	idField := entry.NewRecordField("foo")
	output := &ElasticOutput{
		idField: &idField,
	}

	t.Run("StringValue", func(t *testing.T) {
		entry := entry.New()
		entry.Set(idField, "testval")
		idx, err := output.FindID(entry)
		require.NoError(t, err)
		require.Equal(t, "testval", idx)
	})

	t.Run("ByteValue", func(t *testing.T) {
		entry := entry.New()
		entry.Set(idField, []byte("testval"))
		idx, err := output.FindID(entry)
		require.NoError(t, err)
		require.Equal(t, "testval", idx)
	})

	t.Run("NoValue", func(t *testing.T) {
		entry := entry.New()
		_, err := output.FindID(entry)
		require.Error(t, err)
	})

	t.Run("IDFieldUnset", func(t *testing.T) {
		entry := entry.New()
		output := &ElasticOutput{}
		idx, err := output.FindID(entry)
		require.NoError(t, err)
		require.NotEmpty(t, idx)
	})
}

func TestElastic(t *testing.T) {
	received := make(chan []byte, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			panic(err)
		}
		received <- body
		w.WriteHeader(200)
	}))
	defer ts.Close()

	cfg := NewElasticOutputConfig("test")
	cfg.Addresses = []string{ts.URL}

	ops, err := cfg.Build(testutil.NewBuildContext(t))
	require.NoError(t, err)
	op := ops[0]
	e := entry.New()
	e.Record = "test"

	require.NoError(t, op.Start())
	op.Process(context.Background(), e)
	select {
	case <-time.After(5 * time.Second):
		require.FailNow(t, "Timed out waiting for request")
	case body := <-received:
		dec := json.NewDecoder(bytes.NewReader(body))

		var meta map[string]map[string]interface{}
		err := dec.Decode(&meta)
		require.NoError(t, err)

		var entry map[string]interface{}
		err = dec.Decode(&entry)
		require.NoError(t, err)

		require.Equal(t, "default", meta["index"]["_index"])
		require.Equal(t, float64(0), entry["severity"])
		require.Equal(t, "test", entry["record"])
	}
}
