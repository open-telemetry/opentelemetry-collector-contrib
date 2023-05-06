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

package dimensions

import (
	"context"
	"encoding/json"
	"log"
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

func waitForDims(dimCh <-chan dim, count, waitSeconds int) []dim { // nolint: unparam
	var dims []dim
	timeout := time.After(time.Duration(waitSeconds) * time.Second)

loop:
	for {
		select {
		case d := <-dimCh:
			dims = append(dims, d)
			if len(dims) >= count {
				break loop
			}
		case <-timeout:
			break loop
		}
	}

	return dims
}

func makeHandler(dimCh chan<- dim, forcedResp *atomic.Int32) http.HandlerFunc {
	forcedResp.Store(200)

	return func(rw http.ResponseWriter, r *http.Request) {
		forcedRespInt := int(forcedResp.Load())
		if forcedRespInt != 200 {
			rw.WriteHeader(forcedRespInt)
			return
		}

		log.Printf("Test server got request: %s", r.URL.Path)

		if r.Method != "PATCH" {
			rw.WriteHeader(404)
			return
		}

		match := patchPathRegexp.FindStringSubmatch(r.URL.Path)
		if match == nil {
			rw.WriteHeader(404)
			return
		}

		var bodyDim dim
		if err := json.NewDecoder(r.Body).Decode(&bodyDim); err != nil {
			rw.WriteHeader(400)
			return
		}
		bodyDim.Key = match[1]
		bodyDim.Value = match[2]

		dimCh <- bodyDim

		rw.WriteHeader(200)
	}
}

func setup(t *testing.T) (*DimensionClient, chan dim, *atomic.Int32, context.CancelFunc) {
	dimCh := make(chan dim)

	forcedResp := &atomic.Int32{}
	server := httptest.NewServer(makeHandler(dimCh, forcedResp))

	serverURL, err := url.Parse(server.URL)
	require.NoError(t, err, "failed to get server URL", err)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-ctx.Done()
		server.Close()
	}()

	client := NewDimensionClient(ctx, DimensionClientOptions{
		APIURL:      serverURL,
		LogUpdates:  true,
		Logger:      zap.NewNop(),
		SendDelay:   time.Second,
		MaxBuffered: 10,
	})
	client.Start()

	return client, dimCh, forcedResp, cancel
}

func TestDimensionClient(t *testing.T) {
	client, dimCh, forcedResp, cancel := setup(t)
	defer cancel()

	t.Run("send dimension update with properties and tags", func(t *testing.T) {
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

		dims := waitForDims(dimCh, 1, 3)
		require.Equal(t, dims, []dim{
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
		})
	})

	t.Run("same dimension with different values", func(t *testing.T) {
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

		dims := waitForDims(dimCh, 1, 3)
		require.Equal(t, dims, []dim{
			{
				Key:   "host",
				Value: "test-box",
				Properties: map[string]*string{
					"c": newString("f"),
				},
				TagsToRemove: []string{"active"},
			},
		})
	})

	t.Run("send a distinct prop/tag set for existing dim with server error", func(t *testing.T) {
		forcedResp.Store(500)

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
		dims := waitForDims(dimCh, 1, 3)
		require.Len(t, dims, 0)

		forcedResp.Store(200)
		dims = waitForDims(dimCh, 1, 3)

		// After the server recovers the dim should be resent.
		require.Equal(t, dims, []dim{
			{
				Key:   "AWSUniqueID",
				Value: "abcd",
				Properties: map[string]*string{
					"c": newString("d"),
				},
				Tags: []string{"running"},
			},
		})
	})

	t.Run("does not retry 4xx responses", func(t *testing.T) {
		forcedResp.Store(400)

		// send a distinct prop/tag set for same dim with an error
		require.NoError(t, client.acceptDimension(&DimensionUpdate{
			Name:  "AWSUniqueID",
			Value: "aslfkj",
			Properties: map[string]*string{
				"z": newString("y"),
			},
		}))
		dims := waitForDims(dimCh, 1, 3)
		require.Len(t, dims, 0)

		forcedResp.Store(200)
		dims = waitForDims(dimCh, 1, 3)
		require.Len(t, dims, 0)
	})

	t.Run("does retry 404 responses", func(t *testing.T) {
		forcedResp.Store(404)

		// send a distinct prop/tag set for same dim with an error
		require.NoError(t, client.acceptDimension(&DimensionUpdate{
			Name:  "AWSUniqueID",
			Value: "id404",
			Properties: map[string]*string{
				"z": newString("x"),
			},
		}))

		dims := waitForDims(dimCh, 1, 3)
		require.Len(t, dims, 0)

		forcedResp.Store(200)
		dims = waitForDims(dimCh, 1, 3)
		require.Equal(t, dims, []dim{
			{
				Key:   "AWSUniqueID",
				Value: "id404",
				Properties: map[string]*string{
					"z": newString("x"),
				},
			},
		})
	})

	t.Run("send successive quick updates to same dim", func(t *testing.T) {
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

		dims := waitForDims(dimCh, 1, 3)

		require.Equal(t, dims, []dim{
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
		})
	})
}

func TestFlappyUpdates(t *testing.T) {
	client, dimCh, _, cancel := setup(t)
	defer cancel()

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

	dims := waitForDims(dimCh, 2, 3)
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
	}, dims)
}

func TestInvalidUpdatesNotSent(t *testing.T) {
	client, dimCh, _, cancel := setup(t)
	defer cancel()
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

	dims := waitForDims(dimCh, 2, 3)
	require.Len(t, dims, 0)
}

func newString(s string) *string {
	out := s
	return &out
}
