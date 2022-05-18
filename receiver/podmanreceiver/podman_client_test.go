// Copyright  The OpenTelemetry Authors
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

package podmanreceiver

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func tmpSock(t *testing.T) (net.Listener, string) {
	f, err := ioutil.TempFile(os.TempDir(), "testsock")
	if err != nil {
		t.Fatal(err)
	}
	addr := f.Name()
	os.Remove(addr)

	listener, err := net.Listen("unix", addr)
	if err != nil {
		t.Fatal(err)
	}

	return listener, addr
}

func TestWatchingTimeouts(t *testing.T) {
	listener, addr := tmpSock(t)
	defer listener.Close()
	defer os.Remove(addr)

	config := &Config{
		Endpoint: fmt.Sprintf("unix://%s", addr),
		Timeout:  50 * time.Millisecond,
	}

	cli, err := newPodmanClient(zap.NewNop(), config)
	assert.NotNil(t, cli)
	assert.Nil(t, err)

	expectedError := "context deadline exceeded"

	shouldHaveTaken := time.Now().Add(100 * time.Millisecond).UnixNano()

	err = cli.ping(context.Background())
	require.Error(t, err)

	containers, err := cli.stats(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), expectedError)
	assert.Nil(t, containers)

	assert.GreaterOrEqual(
		t, time.Now().UnixNano(), shouldHaveTaken,
		"Client timeouts don't appear to have been exercised.",
	)
}

func TestStats(t *testing.T) {
	// stats sample
	statsExample := `{"Error":null,"Stats":[{"AvgCPU":42.04781177856639,"ContainerID":"e6af5805edae6c950003abd5451808b277b67077e400f0a6f69d01af116ef014","Name":"charming_sutherland","PerCPU":null,"CPU":42.04781177856639,"CPUNano":309165846000,"CPUSystemNano":54515674,"SystemNano":1650912926385978706,"MemUsage":27717632,"MemLimit":7942234112,"MemPerc":0.34899036730888044,"NetInput":430,"NetOutput":330,"BlockInput":0,"BlockOutput":0,"PIDs":118,"UpTime":309165846000,"Duration":309165846000}]}`

	listener, addr := tmpSock(t)
	defer listener.Close()
	defer os.Remove(addr)

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/stats") {
			_, err := w.Write([]byte(statsExample))
			assert.NoError(t, err)
		} else {
			_, err := w.Write([]byte{})
			assert.NoError(t, err)
		}
	}))
	srv.Listener = listener
	srv.Start()
	defer srv.Close()

	config := &Config{
		Endpoint: fmt.Sprintf("unix://%s", addr),
		// default timeout
		Timeout: 5 * time.Second,
	}

	cli, err := newPodmanClient(zap.NewNop(), config)
	assert.NotNil(t, cli)
	assert.Nil(t, err)

	expectedStats := containerStats{
		AvgCPU:        42.04781177856639,
		ContainerID:   "e6af5805edae6c950003abd5451808b277b67077e400f0a6f69d01af116ef014",
		Name:          "charming_sutherland",
		PerCPU:        nil,
		CPU:           42.04781177856639,
		CPUNano:       309165846000,
		CPUSystemNano: 54515674,
		SystemNano:    1650912926385978706,
		MemUsage:      27717632,
		MemLimit:      7942234112,
		MemPerc:       0.34899036730888044,
		NetInput:      430,
		NetOutput:     330,
		BlockInput:    0,
		BlockOutput:   0,
		PIDs:          118,
		UpTime:        309165846000 * time.Nanosecond,
		Duration:      309165846000,
	}

	stats, err := cli.stats(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, expectedStats, stats[0])
}

func TestStatsError(t *testing.T) {
	// If the stats request fails, the API returns the following Error structure: https://docs.podman.io/en/latest/_static/api.html#operation/ContainersStatsAllLibpod
	// For example, if we query the stats with an invalid container ID, the API returns the following message
	statsError := `{"Error":{},"Stats":null}`

	listener, addr := tmpSock(t)
	defer listener.Close()
	defer os.Remove(addr)

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/stats") {
			_, err := w.Write([]byte(statsError))
			assert.NoError(t, err)
		} else {
			_, err := w.Write([]byte{})
			assert.NoError(t, err)
		}
	}))
	srv.Listener = listener
	srv.Start()
	defer srv.Close()

	config := &Config{
		Endpoint: fmt.Sprintf("unix://%s", addr),
		// default timeout
		Timeout: 5 * time.Second,
	}

	cli, err := newPodmanClient(zap.NewNop(), config)
	assert.NotNil(t, cli)
	assert.Nil(t, err)

	stats, err := cli.stats(context.Background())
	assert.Nil(t, stats)
	assert.EqualError(t, err, errNoStatsFound.Error())
}
