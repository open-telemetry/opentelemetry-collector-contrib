// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package generate // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/generate"

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// Input is an operator that generates log entries.
type Input struct {
	helper.InputOperator
	entry  entry.Entry
	count  int
	static bool
	wg     sync.WaitGroup
	cancel context.CancelFunc
}

// Start will start generating log entries.
func (i *Input) Start(_ operator.Persister) error {
	ctx, cancel := context.WithCancel(context.Background())
	i.cancel = cancel

	i.wg.Go(func() {
		n := 0
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			entry := i.entry.Copy()
			if !i.static {
				entry.Timestamp = time.Now()
			}
			err := i.Write(ctx, entry)
			if err != nil {
				i.Logger().Error("failed to write entry", zap.Error(err))
				return
			}

			n++
			if n == i.count {
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

func recursiveMapInterfaceToMapString(m any) any {
	switch m := m.(type) {
	case map[string]any:
		newMap := make(map[string]any)
		for k, v := range m {
			newMap[k] = recursiveMapInterfaceToMapString(v)
		}
		return newMap
	case map[any]any:
		newMap := make(map[string]any)
		for k, v := range m {
			str, ok := k.(string)
			if !ok {
				str = fmt.Sprintf("%v", k)
			}
			newMap[str] = recursiveMapInterfaceToMapString(v)
		}
		return newMap
	default:
		return m
	}
}
