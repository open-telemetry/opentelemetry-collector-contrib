// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package promtailreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/promtailreceiver"

import (
	"context"
	"path"

	"github.com/go-kit/log"
	"github.com/grafana/loki/clients/pkg/promtail/api"
	"github.com/grafana/loki/clients/pkg/promtail/targets"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/promtailreceiver/internal"
)

const (
	operatorType = "promtail_input"
)

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// PromtailInput implements stanza.Operator interface
type PromtailInput struct {
	helper.InputOperator
	config *Config
	app    *app
	cancel context.CancelFunc
}

type app struct {
	manager *targets.TargetManagers
	client  api.EntryHandler
	entries chan api.Entry
	logger  log.Logger
	reg     *internal.Unregisterer
}

func (a *app) Shutdown() {
	if a.manager != nil {
		a.manager.Stop()
	}
	a.client.Stop()

	// Unregister all prometheus metrics that were registered on Start by
	// targets.NewTargetManagers
	a.reg.UnregisterAll()
}

func (p *PromtailInput) Start(_ operator.Persister) error {
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel

	manager, err := targets.NewTargetManagers(
		p.app,
		p.app.reg,
		p.app.logger,
		p.config.Input.PositionsConfig,
		p.app.client,
		p.config.Input.ScrapeConfig,
		&p.config.Input.TargetConfig,
	)

	if err != nil {
		return err
	}
	p.app.manager = manager

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case inputEntry := <-p.app.entries:
				entry, err := p.parsePromtailEntry(inputEntry)
				if err != nil {
					p.Warnw("Failed to parse promtail entry", zap.Error(err))
					continue
				}
				p.Write(ctx, entry)
			}
		}
	}()
	return nil
}

func (p *PromtailInput) Stop() error {
	p.cancel()
	p.app.Shutdown()
	return nil
}

// parsePromtailEntry creates new stanza.Entry from promtail entry
func (p *PromtailInput) parsePromtailEntry(inputEntry api.Entry) (*entry.Entry, error) {
	outputEntry, err := p.NewEntry(inputEntry.Entry.Line)
	if err != nil {
		return nil, err
	}
	outputEntry.Timestamp = inputEntry.Entry.Timestamp

	for key, val := range inputEntry.Labels {
		valStr := string(val)
		keyStr := string(key)
		switch key {
		case "filename":
			outputEntry.AddAttribute("log.file.path", valStr)
			outputEntry.AddAttribute("log.file.name", path.Base(valStr))
		default:
			outputEntry.AddAttribute(keyStr, valStr)
		}
	}
	return outputEntry, nil
}
