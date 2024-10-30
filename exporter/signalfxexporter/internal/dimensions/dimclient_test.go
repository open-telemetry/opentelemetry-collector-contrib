// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dimensions

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

var patchPathRegexp = regexp.MustCompile(`/v2/dimension/([^/]+)/([^/]+)/_/sfxagent`)

type dim struct {
	Key          string             `json:"key"`
	Value        string             `json:"value"`
	Properties   map[string]*string `json:"customProperties"`
	Tags         []string           `json:"tags"`
	TagsToRemove []string           `json:"tagsToRemove"`
}

type testServer struct {
	startCh      chan struct{}
	finishCh     chan struct{}
	acceptedDims []dim
	server       *httptest.Server
	respCode     int
	requestCount *atomic.Int32
}

func (ts *testServer) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	ts.requestCount.Add(1)
	<-ts.startCh

	if ts.respCode != http.StatusOK {
		rw.WriteHeader(ts.respCode)
		ts.finishCh <- struct{}{}
		return
	}

	match := patchPathRegexp.FindStringSubmatch(r.URL.Path)
	if match == nil {
		rw.WriteHeader(http.StatusNotFound)
		ts.finishCh <- struct{}{}
		return
	}

	var bodyDim dim
	if err := json.NewDecoder(r.Body).Decode(&bodyDim); err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		ts.finishCh <- struct{}{}
		return
	}
	bodyDim.Key = match[1]
	bodyDim.Value = match[2]

	ts.acceptedDims = append(ts.acceptedDims, bodyDim)

	ts.finishCh <- struct{}{}
	rw.WriteHeader(http.StatusOK)
}

// startHandling unblocks the server to handle the request and waits until the request is processed.
func (ts *testServer) handleRequest() {
	ts.startCh <- struct{}{}
	<-ts.finishCh
}

func (ts *testServer) shutdown() {
	ts.reset()
	if ts.server != nil {
		ts.server.Close()
	}
}

func (ts *testServer) reset() {
	if ts.startCh != nil {
		close(ts.startCh)
		ts.startCh = make(chan struct{})
	}
	if ts.finishCh != nil {
		close(ts.finishCh)
		ts.finishCh = make(chan struct{})
	}
	ts.acceptedDims = nil
	ts.respCode = http.StatusOK
	ts.requestCount.Store(0)
}

func setupTestClientServer(t *testing.T) (*DimensionClient, *testServer) {
	ts := &testServer{
		startCh:      make(chan struct{}),
		finishCh:     make(chan struct{}),
		respCode:     http.StatusOK,
		requestCount: new(atomic.Int32),
	}
	ts.server = httptest.NewServer(ts)

	serverURL, err := url.Parse(ts.server.URL)
	require.NoError(t, err, "failed to get server URL", err)

	client := NewDimensionClient(
		DimensionClientOptions{
			APIURL:      serverURL,
			LogUpdates:  true,
			Logger:      zap.NewNop(),
			SendDelay:   100 * time.Millisecond,
			MaxBuffered: 10,
		})
	client.Start()

	return client, ts
}

func TestDimensionClient(t *testing.T) {
	client, server := setupTestClientServer(t)
	defer server.shutdown()
	defer client.Shutdown()

	t.Run("send dimension update with properties and tags", func(t *testing.T) {
		server.reset()
		require.NoError(t, client.acceptDimension(&DimensionUpdate{
			Name:  "host",
			Value: "test-box",
			Properties: map[string]*string{
				"a": newString("b"),
				"c": newString("d"),
				"e": nil,
			},
			Tags: map[string]bool{
				"active":     true,
				"terminated": false,
			},
		}))

		server.handleRequest()
		require.Equal(t, []dim{
			{
				Key:   "host",
				Value: "test-box",
				Properties: map[string]*string{
					"a": newString("b"),
					"c": newString("d"),
					"e": nil,
				},
				Tags:         []string{"active"},
				TagsToRemove: []string{"terminated"},
			},
		}, server.acceptedDims)
		require.EqualValues(t, 1, server.requestCount.Load())
	})

	t.Run("same dimension with different values", func(t *testing.T) {
		server.reset()
		require.NoError(t, client.acceptDimension(&DimensionUpdate{
			Name:  "host",
			Value: "test-box",
			Properties: map[string]*string{
				"c": newString("f"),
			},
			Tags: map[string]bool{
				"active": false,
			},
		}))

		server.handleRequest()
		require.Equal(t, []dim{
			{
				Key:   "host",
				Value: "test-box",
				Properties: map[string]*string{
					"c": newString("f"),
				},
				TagsToRemove: []string{"active"},
			},
		}, server.acceptedDims)
		require.EqualValues(t, 1, server.requestCount.Load())
	})

	t.Run("send a distinct prop/tag set for existing dim with server error", func(t *testing.T) {
		server.reset()
		server.respCode = http.StatusInternalServerError

		// send a distinct prop/tag set for same dim with an error
		require.NoError(t, client.acceptDimension(&DimensionUpdate{
			Name:  "AWSUniqueID",
			Value: "abcd",
			Properties: map[string]*string{
				"c": newString("d"),
			},
			Tags: map[string]bool{
				"running": true,
			},
		}))
		server.handleRequest()
		require.Empty(t, server.acceptedDims)

		server.respCode = http.StatusOK
		server.handleRequest()

		// After the server recovers the dim should be resent.
		require.Equal(t, []dim{
			{
				Key:   "AWSUniqueID",
				Value: "abcd",
				Properties: map[string]*string{
					"c": newString("d"),
				},
				Tags: []string{"running"},
			},
		}, server.acceptedDims)
		require.EqualValues(t, 2, server.requestCount.Load())
	})

	t.Run("does not retry 4xx responses", func(t *testing.T) {
		server.reset()
		server.respCode = http.StatusBadRequest

		// send a distinct prop/tag set for same dim with an error
		require.NoError(t, client.acceptDimension(&DimensionUpdate{
			Name:  "AWSUniqueID",
			Value: "aslfkj",
			Properties: map[string]*string{
				"z": newString("y"),
			},
		}))
		server.handleRequest()

		require.Empty(t, server.acceptedDims)

		server.respCode = http.StatusOK

		// there should be no retries
		require.EqualValues(t, 1, server.requestCount.Load())
	})

	t.Run("does retry 404 responses", func(t *testing.T) {
		server.reset()
		server.respCode = http.StatusNotFound

		// send a distinct prop/tag set for same dim with an error
		require.NoError(t, client.acceptDimension(&DimensionUpdate{
			Name:  "AWSUniqueID",
			Value: "id404",
			Properties: map[string]*string{
				"z": newString("x"),
			},
		}))

		server.handleRequest()
		require.Empty(t, server.acceptedDims)

		server.respCode = http.StatusOK
		server.handleRequest()
		require.Equal(t, []dim{
			{
				Key:   "AWSUniqueID",
				Value: "id404",
				Properties: map[string]*string{
					"z": newString("x"),
				},
			},
		}, server.acceptedDims)
		require.EqualValues(t, 2, server.requestCount.Load())
	})

	t.Run("send successive quick updates to same dim", func(t *testing.T) {
		server.reset()

		require.NoError(t, client.acceptDimension(&DimensionUpdate{
			Name:  "AWSUniqueID",
			Value: "abcd",
			Properties: map[string]*string{
				"e": newString("f"),
			},
			Tags: map[string]bool{
				"running": true,
			},
		}))

		require.NoError(t, client.acceptDimension(&DimensionUpdate{
			Name:  "AWSUniqueID",
			Value: "abcd",
			Properties: map[string]*string{
				"e": newString("f"),
				"g": newString("h"),
			},
			Tags: map[string]bool{
				"dev": true,
			},
		}))

		require.NoError(t, client.acceptDimension(&DimensionUpdate{
			Name:  "AWSUniqueID",
			Value: "abcd",
			Properties: map[string]*string{
				"e": newString("h"),
				"g": nil,
			},
			Tags: map[string]bool{
				"running": false,
			},
		}))

		server.handleRequest()

		require.Equal(t, []dim{
			{
				Key:   "AWSUniqueID",
				Value: "abcd",
				Properties: map[string]*string{
					"e": newString("h"),
					"g": nil,
				},
				Tags:         []string{"dev"},
				TagsToRemove: []string{"running"},
			},
		}, server.acceptedDims)
		require.EqualValues(t, 1, server.requestCount.Load())
	})
}

func TestFlappyUpdates(t *testing.T) {
	client, server := setupTestClientServer(t)
	defer server.shutdown()
	defer client.Shutdown()

	// Do some flappy updates
	for i := 0; i < 5; i++ {
		require.NoError(t, client.acceptDimension(&DimensionUpdate{
			Name:  "pod_uid",
			Value: "abcd",
			Properties: map[string]*string{
				"index": newString(strconv.Itoa(i)),
			},
		}))

		require.NoError(t, client.acceptDimension(&DimensionUpdate{
			Name:  "pod_uid",
			Value: "efgh",
			Properties: map[string]*string{
				"index": newString(strconv.Itoa(i)),
			},
		}))
	}

	// handle 2 requests
	server.handleRequest()
	server.handleRequest()

	require.ElementsMatch(t, []dim{
		{
			Key:        "pod_uid",
			Value:      "abcd",
			Properties: map[string]*string{"index": newString("4")},
		},
		{
			Key:        "pod_uid",
			Value:      "efgh",
			Properties: map[string]*string{"index": newString("4")},
		},
	}, server.acceptedDims)
	require.EqualValues(t, 2, server.requestCount.Load())
}

// TODO: Update the dimension update client to never send empty dimension key or value
func TestInvalidUpdatesNotSent(t *testing.T) {
	t.Skip("This test causes data race because empty dimension key or value result in 404s which causes infinite retries")
	client, server := setupTestClientServer(t)
	defer server.shutdown()
	defer client.Shutdown()
	require.NoError(t, client.acceptDimension(&DimensionUpdate{
		Name:  "host",
		Value: "",
		Properties: map[string]*string{
			"a": newString("b"),
			"c": newString("d"),
		},
		Tags: map[string]bool{
			"active": true,
		},
	}))
	server.handleRequest()

	require.NoError(t, client.acceptDimension(&DimensionUpdate{
		Name:  "",
		Value: "asdf",
		Properties: map[string]*string{
			"a": newString("b"),
			"c": newString("d"),
		},
		Tags: map[string]bool{
			"active": true,
		},
	}))
	server.handleRequest()

	require.EqualValues(t, 2, server.requestCount.Load())
	require.Empty(t, server.acceptedDims)
}

func newString(s string) *string {
	out := s
	return &out
}
