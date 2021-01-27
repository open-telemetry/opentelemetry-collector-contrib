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

package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/opentelemetry/opentelemetry-log-collection/entry"
	"github.com/opentelemetry/opentelemetry-log-collection/operator"
	"github.com/opentelemetry/opentelemetry-log-collection/operator/helper"
)

func init() {
	operator.Register("rate_limit", func() operator.Builder { return NewRateLimitConfig("") })
}

// NewRateLimitConfig creates a new rate limit config with default values
func NewRateLimitConfig(operatorID string) *RateLimitConfig {
	return &RateLimitConfig{
		TransformerConfig: helper.NewTransformerConfig(operatorID, "rate_limit"),
	}
}

// RateLimitConfig is the configuration of a rate filter operator.
type RateLimitConfig struct {
	helper.TransformerConfig `yaml:",inline"`

	Rate     float64         `json:"rate,omitempty"     yaml:"rate,omitempty"`
	Interval helper.Duration `json:"interval,omitempty" yaml:"interval,omitempty"`
	Burst    uint            `json:"burst,omitempty"    yaml:"burst,omitempty"`
}

// Build will build a rate limit operator.
func (c RateLimitConfig) Build(context operator.BuildContext) ([]operator.Operator, error) {
	transformerOperator, err := c.TransformerConfig.Build(context)
	if err != nil {
		return nil, err
	}

	var interval time.Duration
	switch {
	case c.Rate != 0 && c.Interval.Raw() != 0:
		return nil, fmt.Errorf("only one of 'rate' or 'interval' can be defined")
	case c.Rate < 0 || c.Interval.Raw() < 0:
		return nil, fmt.Errorf("rate and interval must be greater than zero")
	case c.Rate > 0:
		interval = time.Second / time.Duration(c.Rate)
	default:
		interval = c.Interval.Raw()
	}

	rateLimitOperator := &RateLimitOperator{
		TransformerOperator: transformerOperator,
		interval:            interval,
		burst:               c.Burst,
	}

	return []operator.Operator{rateLimitOperator}, nil
}

// RateLimitOperator is an operator that limits the rate of log consumption between operators.
type RateLimitOperator struct {
	helper.TransformerOperator

	interval time.Duration
	burst    uint
	isReady  chan struct{}
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// Process will wait until a rate is met before sending an entry to the output.
func (p *RateLimitOperator) Process(ctx context.Context, entry *entry.Entry) error {
	select {
	case <-p.isReady:
		p.Write(ctx, entry)
		return nil
	case <-ctx.Done():
		return nil
	}
}

// Start will start the rate limit operator.
func (p *RateLimitOperator) Start() error {
	p.isReady = make(chan struct{}, p.burst)
	ticker := time.NewTicker(p.interval)

	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel

	// Buffer the ticker ticks in isReady to allow bursts
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer ticker.Stop()
		defer close(p.isReady)

		for {
			select {
			case <-ticker.C:
				p.increment(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (p *RateLimitOperator) increment(ctx context.Context) {
	select {
	case p.isReady <- struct{}{}:
		return
	case <-ctx.Done():
		return
	}
}

// Stop will stop the rate limit operator.
func (p *RateLimitOperator) Stop() error {
	p.cancel()
	p.wg.Wait()
	return nil
}
