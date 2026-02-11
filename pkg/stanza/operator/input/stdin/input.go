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

// Input is an operator that reads input from stdin
type Input struct {
	helper.InputOperator
	wg     sync.WaitGroup
	cancel context.CancelFunc
	stdin  *os.File
}

// Start will start generating log entries.
func (i *Input) Start(_ operator.Persister) error {
	ctx, cancel := context.WithCancel(context.Background())
	i.cancel = cancel

	stat, err := i.stdin.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat stdin: %w", err)
	}

	if stat.Mode()&os.ModeNamedPipe == 0 {
		i.Logger().Warn("No data is being written to stdin")
		return nil
	}

	scanner := bufio.NewScanner(i.stdin)

	i.wg.Go(func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			if ok := scanner.Scan(); !ok {
				if err = scanner.Err(); err != nil {
					i.Logger().Error("Scanning failed", zap.Error(err))
				}
				i.Logger().Info("Stdin has been closed")
				return
			}

			e := entry.New()
			e.Body = scanner.Text()
			err = i.Write(ctx, e)
			if err != nil {
				i.Logger().Error("failed to write entry", zap.Error(err))
				return
			}
		}
	})

	return nil
}

// Stop will stop generating logs.
func (i *Input) Stop() error {
	i.cancel()
	i.wg.Wait()
	return nil
}
