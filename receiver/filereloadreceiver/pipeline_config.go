// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package filereloadereceiver

import (
	"context"
	"fmt"

	"github.com/mitchellh/hashstructure/v2"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/converter/expandconverter"
	"go.opentelemetry.io/collector/confmap/provider/fileprovider"
)

var runnerConfigHashOpt = &hashstructure.HashOptions{IgnoreZeroValue: true}

type runnerConfig struct {
	Receivers  map[config.ComponentID]config.Receiver  `mapstructure:"receivers"`
	Processors map[config.ComponentID]config.Processor `mapstructure:"processors"`

	// Partial pipelines are similar to service pipeline definitions with the difference that partial pipelines
	// do not configure exporters. The remainder of the pipeline definition comes from the parent which is the filereloadreceiver.
	PartialPipelines map[config.ComponentID]*partialPipelineConfig `mapstructure:"partial_pipelines"`

	configType config.Type
	hash       uint64
	path       string
}

type tempRunnerCfg struct {
	Receivers  map[config.ComponentID]map[string]interface{} `mapstructure:"receivers"`
	Processors map[config.ComponentID]map[string]interface{} `mapstructure:"processors"`

	PartialPipelines map[config.ComponentID]*partialPipelineConfig `mapstructure:"partial_pipelines"`
}

type partialPipelineConfig struct {
	Receivers  []config.ComponentID `mapstructure:"receivers"`
	Processors []config.ComponentID `mapstructure:"processors"`
}

func newRunnerConfigFromFile(ctx context.Context, path string, cType config.Type) (*runnerConfig, error) {
	// use a configmap provider to read a runner config
	cm := fileprovider.New()
	r, err := cm.Retrieve(ctx, "file:"+path, nil)
	if err != nil {
		return nil, err
	}

	m, err := r.AsConf()
	if err != nil {
		return nil, err
	}

	err = expandconverter.New().Convert(ctx, m)
	if err != nil {
		return nil, err
	}

	tc := &tempRunnerCfg{}
	err = m.UnmarshalExact(&tc)
	if err != nil {
		return nil, err
	}

	rc := &runnerConfig{path: path}
	rc.PartialPipelines = tc.PartialPipelines
	rc.Receivers, err = unmarshalReceivers(tc.Receivers)
	if err != nil {
		return nil, err
	}

	rc.Processors, err = unmarshalProcessors(tc.Processors)
	if err != nil {
		return nil, err
	}

	rc.configType = cType

	err = rc.Validate()
	if err != nil {
		return nil, err
	}

	// calculate hash once, if we failed to calculate the hash, we will also not be able to
	// stop the runner correctly, thus the error should be returned to prevent it from being
	// started
	rc.hash, err = hashstructure.Hash(rc, hashstructure.FormatV2, runnerConfigHashOpt)
	return rc, err
}

func (r *runnerConfig) Validate() error {
	if len(r.Receivers) == 0 {
		return fmt.Errorf("can't have zero receivers")
	}

	for id, rc := range r.Receivers {
		if err := rc.Validate(); err != nil {
			return fmt.Errorf("receiver %q has an invalid configuration: %w", id, err)
		}

		used := false
		for _, p := range r.PartialPipelines {
			for _, rec := range p.Receivers {
				if rec.String() == rc.ID().String() {
					used = true
					break
				}
			}
		}
		if !used {
			return fmt.Errorf("receiver %q is not used in any pipeline", id)
		}
	}

	for id, p := range r.Processors {
		if err := p.Validate(); err != nil {
			return fmt.Errorf("processor %q has an invalid configuration: %w", id, err)
		}
	}

	if len(r.PartialPipelines) == 0 {
		return fmt.Errorf("can't have zero partial pipelines")
	}

	for id, p := range r.PartialPipelines {
		if id.Type() != r.configType {
			return fmt.Errorf("reloader is supposed to accept only %s pipelines but got %s", r.configType, id.Type())
		}
		for _, ref := range p.Receivers {
			// Check that the name referenced in the pipeline's receivers exists in the top-level receivers.
			if r.Receivers[ref] == nil {
				return fmt.Errorf("pipeline %q references receiver %q which does not exist", id, ref)
			}
		}

		// Validate pipeline processor name references.
		for _, ref := range p.Processors {
			// Check that the name referenced in the pipeline's processors exists in the top-level processors.
			if r.Processors[ref] == nil {
				return fmt.Errorf("pipeline %q references processor %q which does not exist", id, ref)
			}
		}
	}

	return nil
}

func (r *runnerConfig) Hash() uint64 {
	return r.hash
}

func unmarshalReceivers(recvs map[config.ComponentID]map[string]interface{}) (map[config.ComponentID]config.Receiver, error) {
	factories, err := components()
	if err != nil {
		return nil, err
	}

	// Prepare resulting map.
	receivers := make(map[config.ComponentID]config.Receiver)
	// Iterate over input map and create a config for each.
	for id, value := range recvs {
		// Find receiver factory based on "type" that we read from config source.
		factory := factories.Receivers[id.Type()]
		if factory == nil {
			return nil, fmt.Errorf("unknown receiver %s", id.Type())
		}

		// Create the default config for this receiver.
		receiverCfg := factory.CreateDefaultConfig()
		receiverCfg.SetIDName(id.Name())

		// Now that the default config struct is created we can Unmarshal into it,
		// and it will apply user-defined config on top of the default.
		if err := config.UnmarshalReceiver(confmap.NewFromStringMap(value), receiverCfg); err != nil {
			return nil, err
		}

		receivers[id] = receiverCfg
	}

	return receivers, nil
}

func unmarshalProcessors(recvs map[config.ComponentID]map[string]interface{}) (map[config.ComponentID]config.Processor, error) {
	factories, err := components()
	if err != nil {
		return nil, err
	}

	// Prepare resulting map.
	processors := make(map[config.ComponentID]config.Processor)
	// Iterate over input map and create a config for each.
	for id, value := range recvs {
		// Find receiver factory based on "type" that we read from config source.
		factory := factories.Processors[id.Type()]
		if factory == nil {
			return nil, fmt.Errorf("unknown receiver %s", id.Type())
		}

		procCfg := factory.CreateDefaultConfig()
		procCfg.SetIDName(id.Name())

		err = unmarshal(confmap.NewFromStringMap(value), &procCfg)
		if err != nil {
			// LoadReceiver already wraps the error.
			return nil, err
		}

		processors[id] = procCfg
	}

	return processors, nil
}

func unmarshal(componentSection *confmap.Conf, intoCfg interface{}) error {
	if cu, ok := intoCfg.(confmap.Unmarshaler); ok {
		return cu.Unmarshal(componentSection)
	}

	return componentSection.UnmarshalExact(intoCfg)
}
