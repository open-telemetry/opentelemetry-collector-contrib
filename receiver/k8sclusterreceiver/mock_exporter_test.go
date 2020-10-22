// Copyright 2020, OpenTelemetry Authors
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

package k8sclusterreceiver

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/collection"
)

type MockExporter struct {
}

func (m MockExporter) Start(context.Context, component.Host) error {
	return nil
}

func (m MockExporter) Shutdown(context.Context) error {
	return nil
}

var _ component.Exporter = (*mockExporterWithK8sMetadata)(nil)

type mockExporterWithK8sMetadata struct {
	*consumertest.MetricsSink
}

func (m mockExporterWithK8sMetadata) Start(context.Context, component.Host) error {
	return nil
}

func (m mockExporterWithK8sMetadata) Shutdown(context.Context) error {
	return nil
}

func (m mockExporterWithK8sMetadata) ConsumeMetadata([]*collection.MetadataUpdate) error {
	consumeMetadataInvocation()
	return nil
}
