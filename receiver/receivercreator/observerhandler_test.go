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
	"go.opentelemetry.io/collector/config/configmodels"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

type mockRunner struct {
	mock.Mock
}

func (run *mockRunner) start(receiver receiverConfig, discoveredConfig userConfigMap) (component.Receiver, error) {
	args := run.Called(receiver, discoveredConfig)
	return args.Get(0).(component.Receiver), args.Error(1)
}

func (run *mockRunner) shutdown(rcvr component.Receiver) error {
	args := run.Called(rcvr)
	return args.Error(0)
}

var _ runner = (*mockRunner)(nil)

func TestOnAdd(t *testing.T) {
	runner := &mockRunner{}
	rcvrCfg := receiverConfig{typeStr: configmodels.Type("name"), config: userConfigMap{"foo": "bar"}, fullName: "name/1"}
	handler := &observerHandler{
		logger: zap.NewNop(),
		receiverTemplates: map[string]receiverTemplate{
			"name/1": {rcvrCfg, "", newRuleOrPanic(`type.port`)},
		},
		receiversByEndpointID: receiverMap{},
		runner:                runner,
	}

	runner.On("start", rcvrCfg, userConfigMap{endpointConfigKey: "localhost:1234"}).Return(&config.ExampleReceiverProducer{}, nil)

	handler.OnAdd([]observer.Endpoint{
		portEndpoint,
	})

	runner.AssertExpectations(t)
	assert.Equal(t, 1, handler.receiversByEndpointID.Size())
}

func TestOnRemove(t *testing.T) {
	runner := &mockRunner{}
	rcvr := &config.ExampleReceiverProducer{}
	handler := &observerHandler{
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
	rcvrCfg := receiverConfig{typeStr: configmodels.Type("name"), config: userConfigMap{"foo": "bar"}, fullName: "name/1"}
	oldRcvr := &config.ExampleReceiverProducer{}
	newRcvr := &config.ExampleReceiverProducer{}
	handler := &observerHandler{
		logger: zap.NewNop(),
		receiverTemplates: map[string]receiverTemplate{
			"name/1": {rcvrCfg, "", newRuleOrPanic(`type.port`)},
		},
		receiversByEndpointID: receiverMap{},
		runner:                runner,
	}

	handler.receiversByEndpointID.Put("port-1", oldRcvr)

	runner.On("shutdown", oldRcvr).Return(nil)
	runner.On("start", rcvrCfg, userConfigMap{endpointConfigKey: "localhost:1234"}).Return(newRcvr, nil)

	handler.OnChange([]observer.Endpoint{portEndpoint})

	runner.AssertExpectations(t)
	assert.Equal(t, 1, handler.receiversByEndpointID.Size())
	assert.Same(t, newRcvr, handler.receiversByEndpointID.Get("port-1")[0])
}

func TestDynamicConfig(t *testing.T) {
	runner := &mockRunner{}
	handler := &observerHandler{
		logger:                zap.NewNop(),
		receiversByEndpointID: receiverMap{},
		runner:                runner,
		receiverTemplates: map[string]receiverTemplate{
			"name/1": {
				receiverConfig: receiverConfig{typeStr: configmodels.Type("name"), config: userConfigMap{"endpoint": "`endpoint`:6379"}, fullName: "name/1"},
				Rule:           "type.pod",
				rule:           newRuleOrPanic("type.pod"),
			},
		},
	}
	runner.On("start", receiverConfig{
		fullName: "name/1",
		typeStr:  "name",
		config:   userConfigMap{endpointConfigKey: "localhost:6379"},
	}, userConfigMap{}).Return(&config.ExampleReceiverProducer{}, nil)
	handler.OnAdd([]observer.Endpoint{
		podEndpoint,
	})

	runner.AssertExpectations(t)
}
