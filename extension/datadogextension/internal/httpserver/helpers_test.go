// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package httpserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/httpserver"

import (
	"errors"

	"github.com/DataDog/datadog-agent/comp/forwarder/defaultforwarder"
	"github.com/DataDog/datadog-agent/pkg/metrics"
	"github.com/DataDog/datadog-agent/pkg/metrics/event"
	"github.com/DataDog/datadog-agent/pkg/metrics/servicecheck"
	"github.com/DataDog/datadog-agent/pkg/serializer/marshaler"
	"github.com/DataDog/datadog-agent/pkg/serializer/types"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/agentcomponents"
)

var _ agentcomponents.SerializerWithForwarder = (*mockSerializer)(nil)

type mockSerializer struct {
	sendMetadataFunc func(pl any) error
	state            uint32
}

func (m *mockSerializer) SendMetadata(jm marshaler.JSONMarshaler) error {
	if m.sendMetadataFunc != nil {
		return m.sendMetadataFunc(jm)
	}
	return nil
}

func (*mockSerializer) SendEvents(event.Events) error {
	return nil
}

func (*mockSerializer) SendServiceChecks(servicecheck.ServiceChecks) error {
	return nil
}

func (*mockSerializer) SendSeriesWithMetadata(metrics.Series) error {
	return nil
}

func (*mockSerializer) SendIterableSeries(metrics.SerieSource) error {
	return nil
}

func (*mockSerializer) AreSeriesEnabled() bool {
	return false
}

func (*mockSerializer) SendSketch(metrics.SketchesSource) error {
	return nil
}

func (*mockSerializer) AreSketchesEnabled() bool {
	return false
}

func (*mockSerializer) SendHostMetadata(marshaler.JSONMarshaler) error {
	return nil
}

func (*mockSerializer) SendProcessesMetadata(any) error {
	return nil
}

func (*mockSerializer) SendAgentchecksMetadata(marshaler.JSONMarshaler) error {
	return nil
}

func (*mockSerializer) SendOrchestratorMetadata([]types.ProcessMessageBody, string, string, int) error {
	return nil
}

func (*mockSerializer) SendOrchestratorManifests([]types.ProcessMessageBody, string, string) error {
	return nil
}

func (m *mockSerializer) Start() error {
	m.state = defaultforwarder.Started
	return nil
}

func (m *mockSerializer) State() uint32 {
	return m.state
}

func (m *mockSerializer) Stop() {
	m.state = defaultforwarder.Stopped
}

// mockJSONErrorPayload implements marshaler.JSONMarshaler but always fails to marshal
type mockJSONErrorPayload struct{}

func (*mockJSONErrorPayload) MarshalJSON() ([]byte, error) {
	return nil, errors.New("mock marshal error")
}

func (*mockJSONErrorPayload) SplitPayload(int) ([]marshaler.AbstractMarshaler, error) {
	return nil, errors.New("mock split error")
}
