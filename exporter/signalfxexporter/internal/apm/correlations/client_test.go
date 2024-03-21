// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// Originally copied from https://github.com/signalfx/signalfx-agent/blob/fbc24b0fdd3884bd0bbfbd69fe3c83f49d4c0b77/pkg/apm/correlations/client_test.go

package correlations

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/apm/log"
)

var getPathRegexp = regexp.MustCompile(`/v2/apm/correlate/([^/]+)/([^/]+)`)                    // /dimName/dimVal
var putPathRegexp = regexp.MustCompile(`/v2/apm/correlate/([^/]+)/([^/]+)/([^/]+)`)            // /dimName/dimVal/{service,environment}
var deletePathRegexp = regexp.MustCompile(`/v2/apm/correlate/([^/]+)/([^/]+)/([^/]+)/([^/]+)`) // /dimName/dimValue/{service,environment}/value

func waitForCors(corCh <-chan *request, count, waitSeconds int) []*request { // nolint: unparam
	cors := make([]*request, 0, count)
	timeout := time.After(time.Duration(waitSeconds) * time.Second)

loop:
	for {
		select {
		case cor := <-corCh:
			cors = append(cors, cor)
			if len(cors) >= count {
				break loop
			}
		case <-timeout:
			break loop
		}
	}

	return cors
}

func makeHandler(t *testing.T, corCh chan<- *request, forcedRespCode *atomic.Value, forcedRespPayload *atomic.Value) http.HandlerFunc {
	forcedRespCode.Store(200)

	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		forcedRespInt := forcedRespCode.Load().(int)
		if forcedRespInt != 200 {
			rw.WriteHeader(forcedRespInt)
			return
		}

		t.Logf("Test server got %s request: %s", r.Method, r.URL.Path)
		var cor *request
		switch r.Method {
		case http.MethodGet:
			match := getPathRegexp.FindStringSubmatch(r.URL.Path)
			if match == nil || len(match) < 3 {
				rw.WriteHeader(404)
				return
			}
			corCh <- &request{
				operation: r.Method,
				Correlation: &Correlation{
					DimName:  match[1],
					DimValue: match[2],
				},
			}
			_, _ = rw.Write(forcedRespPayload.Load().([]byte))
			return
		case http.MethodPut:
			match := putPathRegexp.FindStringSubmatch(r.URL.Path)
			if match == nil || len(match) < 4 {
				rw.WriteHeader(404)
				return
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				rw.WriteHeader(400)
				return
			}
			cor = &request{
				operation: r.Method,
				Correlation: &Correlation{
					DimName:  match[1],
					DimValue: match[2],
					Type:     Type(match[3]),
					Value:    string(body),
				},
			}

		case http.MethodDelete:
			match := deletePathRegexp.FindStringSubmatch(r.URL.Path)
			if match == nil || len(match) < 5 {
				rw.WriteHeader(404)
				return
			}
			cor = &request{
				operation: r.Method,
				Correlation: &Correlation{
					DimName:  match[1],
					DimValue: match[2],
					Type:     Type(match[3]),
					Value:    match[4],
				},
			}
		default:
			rw.WriteHeader(404)
			return
		}

		corCh <- cor

		rw.WriteHeader(200)
	})
}

func setup(t *testing.T) (CorrelationClient, chan *request, *atomic.Value, *atomic.Value, context.CancelFunc) {
	serverCh := make(chan *request, 100)

	var forcedRespCode atomic.Value
	var forcedRespPayload atomic.Value
	server := httptest.NewServer(makeHandler(t, serverCh, &forcedRespCode, &forcedRespPayload))

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-ctx.Done()
		server.Close()
	}()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		panic(err)
	}

	conf := ClientConfig{
		Config: Config{
			MaxRequests:     10,
			MaxBuffered:     10,
			MaxRetries:      4,
			LogUpdates:      true,
			RetryDelay:      0 * time.Second,
			CleanupInterval: 1 * time.Minute,
		},
		AccessToken: "",
		URL:         serverURL,
	}

	httpClient := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:        int(conf.MaxRequests),
			MaxIdleConnsPerHost: int(conf.MaxRequests),
			IdleConnTimeout:     30 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}

	client, err := NewCorrelationClient(ctx, log.Nil, httpClient, conf)
	if err != nil {
		panic("could not make correlation client: " + err.Error())
	}
	client.Start()

	return client, serverCh, &forcedRespCode, &forcedRespPayload, cancel
}

func TestCorrelationClient(t *testing.T) {
	client, serverCh, forcedRespCode, forcedRespPayload, cancel := setup(t)
	defer close(serverCh)
	defer cancel()

	for _, correlationType := range []Type{Service, Environment} {
		for _, op := range []string{http.MethodPut, http.MethodDelete} {
			op := op
			correlationType := correlationType
			t.Run(fmt.Sprintf("%v %v", op, correlationType), func(t *testing.T) {
				testData := &Correlation{Type: correlationType, DimName: "host", DimValue: "test-box", Value: "test-service"}
				switch op {
				case http.MethodPut:
					client.Correlate(testData, CorrelateCB(func(_ *Correlation, _ error) {}))
				case http.MethodDelete:
					client.Delete(testData, SuccessfulDeleteCB(func(_ *Correlation) {}))
				}
				cors := waitForCors(serverCh, 1, 5)
				require.Equal(t, []*request{{operation: op, Correlation: testData}}, cors)
			})
		}
	}
	t.Run("GET returns expected payload and 200 response", func(t *testing.T) {
		forcedRespCode.Store(200)
		respPayload := map[string][]string{"sf_services": {"testService1"}}
		respJSON, err := json.Marshal(&respPayload)
		require.Nil(t, err, "json marshaling failed in test")
		forcedRespPayload.Store(respJSON)

		var wg sync.WaitGroup
		wg.Add(1)
		var receivedPayload map[string][]string
		client.Get("host", "test-box", SuccessfulGetCB(func(resp map[string][]string) {
			receivedPayload = resp
			wg.Done()
		}))
		wg.Wait()

		require.Equal(t, respPayload, receivedPayload)

		cors := waitForCors(serverCh, 1, 3)
		require.Len(t, cors, 1)
	})
	t.Run("does not retry 4xx responses", func(t *testing.T) {
		forcedRespCode.Store(400)

		testData := &Correlation{Type: Service, DimName: "host", DimValue: "test-box", Value: "test-service"}
		client.Correlate(testData, CorrelateCB(func(_ *Correlation, _ error) {}))

		cors := waitForCors(serverCh, 1, 3)
		require.Len(t, cors, 0)

		forcedRespCode.Store(200)
		cors = waitForCors(serverCh, 1, 3)
		require.Len(t, cors, 0)
	})
	t.Run("does retry 500 responses", func(t *testing.T) {
		forcedRespCode.Store(500)
		require.Equal(t, int64(0), atomic.LoadInt64(&client.(*Client).TotalRetriedUpdates), "test cleaned up correctly")
		testData := &Correlation{Type: Service, DimName: "host", DimValue: "test-box", Value: "test-service"}
		client.Correlate(testData, CorrelateCB(func(_ *Correlation, _ error) {}))
		// sending the testData twice tests deduplication, since the 500 status
		// will trigger retries, and the requests should be deduped and the
		// TotalRertriedUpdates should still only be 5
		client.Correlate(testData, CorrelateCB(func(_ *Correlation, _ error) {}))

		cors := waitForCors(serverCh, 1, 4)
		require.Len(t, cors, 0)
		require.Equal(t, uint32(5), client.(*Client).maxAttempts)
		require.Equal(t, int64(5), atomic.LoadInt64(&client.(*Client).TotalRetriedUpdates))

		forcedRespCode.Store(200)
		cors = waitForCors(serverCh, 1, 2)
		require.Equal(t, []*request{}, cors)
	})
}
