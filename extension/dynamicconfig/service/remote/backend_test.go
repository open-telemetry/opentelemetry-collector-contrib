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

package remote

import (
	"bytes"
	"fmt"
	"testing"

	pb "github.com/open-telemetry/opentelemetry-collector-contrib/extension/dynamicconfig/proto/experimental/metrics/configservice"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/dynamicconfig/service/mock"
)

func SetUpServer(t *testing.T) (*Backend, chan struct{}, chan struct{}) {
	quit := make(chan struct{})
	done := make(chan struct{})

	// making mock third-party
	address := mock.StartServer(t, quit, done)
	<-done

	// making remote backend
	backend, err := NewBackend(address)
	if err != nil {
		t.Fatalf("fail to init remote config backend")
	}

	return backend, quit, done
}

func TearDownServer(t *testing.T, backend *Backend, quit chan struct{}, done chan struct{}) {
	quit <- struct{}{}
	if err := backend.Close(); err != nil {
		t.Errorf("fail to close backend: %v", err)
	}

	<-done
}

func TestNewBackend(t *testing.T) {
	if _, err := NewBackend(":0"); err != nil {
		t.Errorf("fail to initialize a new remote backend: %v", err)
	}
}

func TestBuildConfigResponseRemote(t *testing.T) {
	backend, quit, done := SetUpServer(t)
	defer TearDownServer(t, backend, quit, done)

	resp := buildResp(t, backend)
	if !bytes.Equal(resp.Fingerprint, mock.GlobalResponse.Fingerprint) {
		t.Errorf("expected resp %v, got %v", mock.GlobalResponse, resp)
	}

	newFingerprint := []byte("actually, I believe Gretchen was a cow")
	mock.AlterFingerprint(newFingerprint)

	resp = buildResp(t, backend)
	if !bytes.Equal(resp.Fingerprint, mock.GlobalResponse.Fingerprint) {
		t.Errorf("expected resp %v, got %v", mock.GlobalResponse, resp)
	}
}

func buildResp(t *testing.T, backend *Backend) *pb.MetricConfigResponse {
	resp, err := backend.BuildConfigResponse(nil)
	if err != nil {
		t.Errorf("fail to build config response: %v", err)
	}

	return resp
}

func TestBuildBadResponse(t *testing.T) {
	backend, quit, done := SetUpServer(t)
	defer TearDownServer(t, backend, quit, done)

	mock.GlobalError = fmt.Errorf("something catastrophic has happened!")
	defer mock.ResetError()

	if _, err := backend.BuildConfigResponse(nil); err == nil {
		fmt.Errorf("fail to catch error in backend")
	}
}

func TestDoubleClose(t *testing.T) {
	backend, quit, done := SetUpServer(t)
	TearDownServer(t, backend, quit, done)

	if err := backend.Close(); err == nil {
		t.Errorf("should have failed to close backend, since it's already closed")
	}
}
