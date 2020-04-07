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

package receivercreator

import (
	"context"
	"errors"

	"github.com/jwangsadinata/go-multimap/setmultimap"
	"github.com/open-telemetry/opentelemetry-collector/component"
	"github.com/open-telemetry/opentelemetry-collector/consumer"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

var (
	errNilNextConsumer = errors.New("nil nextConsumer")
)

var _ component.MetricsReceiver = (*receiverCreator)(nil)

// receiverCreator implements consumer.MetricsConsumer.
type receiverCreator struct {
	nextConsumer consumer.MetricsConsumerOld
	logger       *zap.Logger
	cfg          *Config
	responder    *observerHandler
	observer     observer.Observable
}

// newReceiverCreator creates the receiver_creator with the given parameters.
func newReceiverCreator(logger *zap.Logger, nextConsumer consumer.MetricsConsumerOld, cfg *Config) (component.MetricsReceiver, error) {
	if nextConsumer == nil {
		return nil, errNilNextConsumer
	}

	r := &receiverCreator{
		logger:       logger,
		nextConsumer: nextConsumer,
		cfg:          cfg,
	}
	return r, nil
}

// Start receiver_creator.
func (rc *receiverCreator) Start(ctx context.Context, host component.Host) error {
	rc.responder = &observerHandler{
		logger:                rc.logger,
		receiverTemplates:     rc.cfg.receiverTemplates,
		receiversByEndpointID: setmultimap.New(),
		runner: &receiverRunner{
			logger:       rc.logger,
			nextConsumer: rc.nextConsumer,
			idNamespace:  rc.cfg.Name(),
			// TODO: not really sure what context should be used here for starting subreceivers
			// as don't think it makes sense to use Start context as the lifetimes are different.
			ctx:  context.Background(),
			host: host,
		}}

	rc.observer.ListAndWatch(rc.responder)

	return nil
}

// Shutdown stops the receiver_creator and all its receivers started at runtime.
func (rc *receiverCreator) Shutdown(ctx context.Context) error {
	return rc.responder.Shutdown()
}
