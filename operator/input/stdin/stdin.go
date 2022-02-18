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

package stdin

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sync"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/operator"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/helper"
)

func init() {
	operator.Register("stdin", func() operator.Builder { return NewStdinInputConfig("") })
}

// NewStdinInputConfig creates a new stdin input config with default values
func NewStdinInputConfig(operatorID string) *StdinInputConfig {
	return &StdinInputConfig{
		InputConfig: helper.NewInputConfig(operatorID, "stdin"),
	}
}

// StdinInputConfig is the configuration of a stdin input operator.
type StdinInputConfig struct {
	helper.InputConfig `yaml:",inline"`
}

// Build will build a stdin input operator.
func (c *StdinInputConfig) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	inputOperator, err := c.InputConfig.Build(logger)
	if err != nil {
		return nil, err
	}

	return &StdinInput{
		InputOperator: inputOperator,
		stdin:         os.Stdin,
	}, nil
}

// StdinInput is an operator that reads input from stdin
type StdinInput struct {
	helper.InputOperator
	wg     sync.WaitGroup
	cancel context.CancelFunc
	stdin  *os.File
}

// Start will start generating log entries.
func (g *StdinInput) Start(_ operator.Persister) error {
	ctx, cancel := context.WithCancel(context.Background())
	g.cancel = cancel

	stat, err := g.stdin.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat stdin: %s", err)
	}

	if stat.Mode()&os.ModeNamedPipe == 0 {
		g.Warn("No data is being written to stdin")
		return nil
	}

	scanner := bufio.NewScanner(g.stdin)

	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			if ok := scanner.Scan(); !ok {
				if err := scanner.Err(); err != nil {
					g.Errorf("Scanning failed", zap.Error(err))
				}
				g.Infow("Stdin has been closed")
				return
			}

			e := entry.New()
			e.Body = scanner.Text()
			g.Write(ctx, e)
		}
	}()

	return nil
}

// Stop will stop generating logs.
func (g *StdinInput) Stop() error {
	g.cancel()
	g.wg.Wait()
	return nil
}
