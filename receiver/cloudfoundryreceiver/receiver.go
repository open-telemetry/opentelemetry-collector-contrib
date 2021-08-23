// Copyright 2019, OpenTelemetry Authors
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

package cloudfoundryreceiver

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
)

var _ component.MetricsReceiver = (*cloudFoundryReceiver)(nil)

// newCloudFoundryReceiver implements the component.MetricsReceiver for Cloud Foundry protocol.
// todo implement - currently dummy for initial PR that only implements config and factory
type cloudFoundryReceiver struct {
}

// newCloudFoundryReceiver creates the Cloud Foundry receiver with the given parameters.
// todo implement - currently dummy for initial PR that only implements config and factory
func newCloudFoundryReceiver(
	_ *zap.Logger,
	_ Config,
	nextConsumer consumer.Metrics) (component.MetricsReceiver, error) {

	if nextConsumer == nil {
		return nil, componenterror.ErrNilNextConsumer
	}

	return &cloudFoundryReceiver{}, nil
}

func (cfr *cloudFoundryReceiver) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (cfr *cloudFoundryReceiver) Shutdown(_ context.Context) error {
	return nil
}
