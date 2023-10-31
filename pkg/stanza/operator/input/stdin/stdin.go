// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stdin // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/stdin"

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sync"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

func init() {
	operator.Register("stdin", func() operator.Builder { return NewConfig("") })
}

// NewConfig creates a new stdin input config with default values
func NewConfig(operatorID string) *Config {
	return &Config{
		InputConfig: helper.NewInputConfig(operatorID, "stdin"),
	}
}

// Config is the configuration of a stdin input operator.
type Config struct {
	helper.InputConfig `mapstructure:",squash"`
}

// Build will build a stdin input operator.
func (c *Config) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	inputOperator, err := c.InputConfig.Build(logger)
	if err != nil {
		return nil, err
	}

	return &Input{
		InputOperator: inputOperator,
		stdin:         os.Stdin,
	}, nil
}

// Input is an operator that reads input from stdin
type Input struct {
	helper.InputOperator
	wg     sync.WaitGroup
	cancel context.CancelFunc
	stdin  *os.File
}

// Start will start generating log entries.
func (g *Input) Start(_ operator.Persister) error {
	ctx, cancel := context.WithCancel(context.Background())
	g.cancel = cancel

	stat, err := g.stdin.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat stdin: %w", err)
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
func (g *Input) Stop() error {
	g.cancel()
	g.wg.Wait()
	return nil
}
