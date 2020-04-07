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
	"fmt"
	"sync"

	"github.com/jwangsadinata/go-multimap"
	"github.com/open-telemetry/opentelemetry-collector/component"
	"github.com/open-telemetry/opentelemetry-collector/component/componenterror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

var _ observer.Notify = (*observerHandler)(nil)

// observerHandler manages endpoint change notifications.
type observerHandler struct {
	sync.Mutex
	logger *zap.Logger
	// receiverTemplates maps receiver template full name to a receiverTemplate value.
	receiverTemplates map[string]receiverTemplate
	// receiversByEndpointID is a map of endpoint IDs to a receiver instance.
	receiversByEndpointID multimap.MultiMap
	// runner starts and stops receiver instances.
	runner runner
}

// Shutdown all receivers started at runtime.
func (re *observerHandler) Shutdown() error {
	re.Lock()
	defer re.Unlock()

	var errs []error

	for _, rcvr := range re.receiversByEndpointID.Values() {
		rcvr := rcvr.(component.Receiver)

		if err := re.runner.shutdown(rcvr); err != nil {
			// TODO: Should keep track of which receiver the error is associated with
			// but require some restructuring.
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("shutdown on %d receivers failed: %v", len(errs), componenterror.CombineErrors(errs))
	}

	return nil
}

// OnAdd responds to endpoint add notifications.
func (re *observerHandler) OnAdd(added []observer.Endpoint) {
	re.Lock()
	defer re.Unlock()

	for _, e := range added {
		for _, template := range re.receiverTemplates {
			if !ruleMatches(template.Rule, e) {
				continue
			}
			rcvr, err := re.runner.start(template.receiverConfig, userConfigMap{
				"endpoint": e.Target(),
			})
			if err != nil {
				re.logger.Error("failed to start receiver", zap.String("receiver", template.fullName))
				continue
			}

			re.receiversByEndpointID.Put(e.ID(), rcvr)
		}
	}
}

// OnRemove responds to endpoint removal notifications.
func (re *observerHandler) OnRemove(removed []observer.Endpoint) {
	re.Lock()
	defer re.Unlock()

	for _, e := range removed {
		entries, _ := re.receiversByEndpointID.Get(e.ID())
		for _, rcvr := range entries {
			rcvr := rcvr.(component.Receiver)
			if err := re.runner.shutdown(rcvr); err != nil {
				re.logger.Error("failed to stop receiver", zap.Reflect("receiver", rcvr))
				continue
			}
		}
		re.receiversByEndpointID.RemoveAll(e.ID())
	}
}

// OnChange responds to endpoint change notifications.
func (re *observerHandler) OnChange(changed []observer.Endpoint) {
	// TODO: optimize to only restart if effective config has changed.
	re.OnRemove(changed)
	re.OnAdd(changed)
}
