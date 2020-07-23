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

package service

import (
	"bytes"
	"context"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/dynamicconfig/service/file"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/dynamicconfig/service/mock"
	pb "github.com/open-telemetry/opentelemetry-proto/gen/go/experimental/metricconfigservice"
)

func TestNewConfigService(t *testing.T) {
	if service, err := NewConfigService(); service != nil || err == nil {
		t.Errorf("no backend specified but service created: %v: %v", service, err)
	}

	if service, err := NewConfigService(WithMockBackend()); service == nil || err != nil {
		t.Errorf("backend specified but service not created: %v: %v", service, err)
	}
}

func TestLocalConfigOption(t *testing.T) {
	if service, err := NewConfigService(WithFileConfig("woot.yaml")); service != nil || err == nil {
		t.Errorf("file does not exist but service created: %v: %v", service, err)
	}

	service, err := NewConfigService(WithFileConfig("../testdata/schedules.yaml"))
	if service == nil || err != nil {
		t.Errorf("file exists but service not created: %v: %v", service, err)
	}
}

func TestWaitTimeConfigOption(t *testing.T) {
	const testWaitTime = 60

	service, err := NewConfigService(
		WithFileConfig("../testdata/schedules.yaml"),
		WithWaitTime(testWaitTime),
	)
	if service == nil || err != nil {
		t.Errorf("file exists but service not created: %v: %v", service, err)
	}

	time := service.backend.(*file.Backend).GetWaitTime()
	if time != testWaitTime {
		t.Errorf("wait time of %d requested, found %d", testWaitTime, time)
	}

}

func TestGetMetricConfig(t *testing.T) {
	service, err := NewConfigService(WithMockBackend())
	sameFingerprintReq := pb.MetricConfigRequest{LastKnownFingerprint: mock.GlobalFingerprint}

	resp, err := service.GetMetricConfig(context.Background(), &sameFingerprintReq)
	if err != nil {
		t.Errorf("failed to get config: %v", err)
	}

	if !bytes.Equal(resp.Fingerprint, mock.GlobalFingerprint) {
		t.Errorf("expected fingerprint to equal %v: got %v", mock.GlobalFingerprint, resp.Fingerprint)
	}

	blankReq := pb.MetricConfigRequest{}
	resp, err = service.GetMetricConfig(context.Background(), &blankReq)
	if err != nil {
		t.Errorf("failed to get config: %v", err)
	}
}
