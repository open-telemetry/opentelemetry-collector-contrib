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

package loadbalancingexporter

import (
	"context"
	"sort"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

var _ component.TraceExporter = (*exporterImp)(nil)

const (
	defaultResInterval = 5 * time.Second
	defaultResTimeout  = time.Second
)

type exporterImp struct {
	logger      *zap.Logger
	config      Config
	ring        *hashRing
	res         resolver
	resInterval time.Duration
	resTimeout  time.Duration
	stopped     bool
	shutdownWg  sync.WaitGroup
}

// Crete new exporter
func newExporter(logger *zap.Logger, cfg configmodels.Exporter) (*exporterImp, error) {
	oCfg := cfg.(*Config)

	return &exporterImp{
		logger:      logger,
		config:      *oCfg,
		resInterval: defaultResInterval,
		resTimeout:  defaultResTimeout,
	}, nil
}

func (e *exporterImp) Start(ctx context.Context, host component.Host) error {
	err := e.resolveAndUpdate(ctx)
	if err != nil {
		return err
	}

	e.shutdownWg.Add(1)
	go e.periodicallyResolve()

	return nil
}

func (e *exporterImp) periodicallyResolve() {
	if e.stopped {
		e.shutdownWg.Done()
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), e.resTimeout)
	defer cancel()

	if err := e.resolveAndUpdate(ctx); err != nil {
		e.logger.Debug("failed to resolve endpoints", zap.Error(err))
	}

	time.AfterFunc(e.resInterval, func() {
		e.periodicallyResolve()
	})
}

func (e *exporterImp) resolveAndUpdate(ctx context.Context) error {
	resolved, err := e.res.resolve(ctx)
	if err != nil {
		return err
	}
	resolved = sort.StringSlice(resolved)
	newRing := newHashRing(resolved)

	if !newRing.equal(e.ring) {
		atomic.SwapPointer((*unsafe.Pointer)(unsafe.Pointer(&e.ring)), unsafe.Pointer(newRing))
	}

	return nil
}

func (e *exporterImp) Shutdown(context.Context) error {
	e.stopped = true
	e.shutdownWg.Wait()
	return nil
}

func (e *exporterImp) ConsumeTraces(ctx context.Context, td pdata.Traces) error {
	return nil
}

func (e *exporterImp) GetCapabilities() component.ProcessorCapabilities {
	return component.ProcessorCapabilities{MutatesConsumedData: false}
}
