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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

type mockRunner struct {
	mock.Mock
}

func (run *mockRunner) start(
	receiver receiverConfig,
	discoveredConfig userConfigMap,
	nextConsumer consumer.Metrics,
) (component.Receiver, error) {
	args := run.Called(receiver, discoveredConfig, nextConsumer)
	return args.Get(0).(component.Receiver), args.Error(1)
}

func (run *mockRunner) shutdown(rcvr component.Receiver) error {
	args := run.Called(rcvr)
	return args.Error(0)
}

var _ runner = (*mockRunner)(nil)

func TestOnAdd(t *testing.T) {
	runner := &mockRunner{}
	rcvrCfg := receiverConfig{id: config.NewComponentIDWithName("name", "1"), config: userConfigMap{"foo": "bar"}}
	cfg := createDefaultConfig().(*Config)
	cfg.receiverTemplates = map[string]receiverTemplate{
		"name/1": {rcvrCfg, "", newRuleOrPanic(`type == "port"`)},
	}
	handler := &observerHandler{
		config:                cfg,
		logger:                zap.NewNop(),
		receiversByEndpointID: receiverMap{},
		runner:                runner,
	}

	runner.On(
		"start",
		rcvrCfg,
		userConfigMap{endpointConfigKey: "localhost:1234"},
		mock.IsType(&resourceEnhancer{}),
	).Return(&nopWithEndpointReceiver{}, nil)

	handler.OnAdd([]observer.Endpoint{
		portEndpoint,
		unsupportedEndpoint,
	})

	runner.AssertExpectations(t)
	assert.Equal(t, 1, handler.receiversByEndpointID.Size())
}

func TestOnRemove(t *testing.T) {
	runner := &mockRunner{}
	rcvr := &nopWithEndpointReceiver{}
	handler := &observerHandler{
		config:                createDefaultConfig().(*Config),
		logger:                zap.NewNop(),
		receiversByEndpointID: receiverMap{},
		runner:                runner,
	}

	handler.receiversByEndpointID.Put("port-1", rcvr)

	runner.On("shutdown", rcvr).Return(nil)

	handler.OnRemove([]observer.Endpoint{portEndpoint})

	runner.AssertExpectations(t)
	assert.Equal(t, 0, handler.receiversByEndpointID.Size())
}

func TestOnChange(t *testing.T) {
	runner := &mockRunner{}
	rcvrCfg := receiverConfig{id: config.NewComponentIDWithName("name", "1"), config: userConfigMap{"foo": "bar"}}
	oldRcvr := &nopWithEndpointReceiver{}
	newRcvr := &nopWithEndpointReceiver{}
	cfg := createDefaultConfig().(*Config)
	cfg.receiverTemplates = map[string]receiverTemplate{
		"name/1": {rcvrCfg, "", newRuleOrPanic(`type == "port"`)},
	}
	handler := &observerHandler{
		config:                cfg,
		logger:                zap.NewNop(),
		receiversByEndpointID: receiverMap{},
		runner:                runner,
	}

	handler.receiversByEndpointID.Put("port-1", oldRcvr)

	runner.On("shutdown", oldRcvr).Return(nil)
	runner.On(
		"start",
		rcvrCfg,
		userConfigMap{endpointConfigKey: "localhost:1234"},
		mock.IsType(&resourceEnhancer{}),
	).Return(newRcvr, nil)

	handler.OnChange([]observer.Endpoint{portEndpoint})

	runner.AssertExpectations(t)
	assert.Equal(t, 1, handler.receiversByEndpointID.Size())
	assert.Same(t, newRcvr, handler.receiversByEndpointID.Get("port-1")[0])
}

func TestDynamicConfig(t *testing.T) {
	runner := &mockRunner{}
	cfg := createDefaultConfig().(*Config)
	cfg.receiverTemplates = map[string]receiverTemplate{
		"name/1": {
			receiverConfig: receiverConfig{id: config.NewComponentIDWithName("name", "1"), config: userConfigMap{"endpoint": "`endpoint`:6379"}},
			Rule:           `type == "pod"`,
			rule:           newRuleOrPanic("type == \"pod\""),
		},
	}
	handler := &observerHandler{
		config:                cfg,
		logger:                zap.NewNop(),
		receiversByEndpointID: receiverMap{},
		runner:                runner,
	}
	runner.On(
		"start",
		receiverConfig{
			id:     config.NewComponentIDWithName("name", "1"),
			config: userConfigMap{endpointConfigKey: "localhost:6379"},
		},
		userConfigMap{},
		mock.IsType(&resourceEnhancer{}),
	).Return(&nopWithEndpointReceiver{}, nil)
	handler.OnAdd([]observer.Endpoint{
		podEndpoint,
	})

	runner.AssertExpectations(t)
}
