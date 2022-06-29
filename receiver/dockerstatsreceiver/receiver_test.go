// Copyright 2020 OpenTelemetry Authors
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

//go:build !windows
// +build !windows

// TODO review if tests should succeed on Windows

package dockerstatsreceiver

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

func newMockServer(tb testing.TB, responseBodyFile string) *httptest.Server {
	file, err := os.Open(responseBodyFile)
	assert.NoError(tb, err)
	fileContents, err := ioutil.ReadAll(file)
	require.NoError(tb, err)
	return httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusOK)
		_, err := rw.Write(fileContents)
		require.NoError(tb, err)
		return
	}))
}

func TestNewReceiver(t *testing.T) {
	cfg := &Config{
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			CollectionInterval: 1 * time.Second,
		},
		Endpoint:         "unix:///run/some.sock",
		DockerAPIVersion: defaultDockerAPIVersion,
	}
	mr := newReceiver(componenttest.NewNopReceiverCreateSettings(), cfg)
	assert.NotNil(t, mr)
}

func TestErrorsInStart(t *testing.T) {
	unreachable := "unix:///not/a/thing.sock"
	cfg := &Config{
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			CollectionInterval: 1 * time.Second,
		},
		Endpoint:         unreachable,
		DockerAPIVersion: defaultDockerAPIVersion,
	}
	recv := newReceiver(componenttest.NewNopReceiverCreateSettings(), cfg)
	assert.NotNil(t, recv)

	cfg.Endpoint = "..not/a/valid/endpoint"
	err := recv.start(context.Background(), componenttest.NewNopHost())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unable to parse docker host")

	cfg.Endpoint = unreachable
	err = recv.start(context.Background(), componenttest.NewNopHost())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}
