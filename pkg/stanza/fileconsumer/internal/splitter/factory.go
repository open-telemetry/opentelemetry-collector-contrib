// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splitter // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/splitter"

import (
	"bufio"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/flush"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"
)

type Factory interface {
	SplitFunc() bufio.SplitFunc
}

type factory struct {
	splitFunc   bufio.SplitFunc
	trimFunc    trim.Func
	flushPeriod time.Duration
}

var _ Factory = (*factory)(nil)

func NewFactory(splitFunc bufio.SplitFunc, trimFunc trim.Func, flushPeriod time.Duration) Factory {
	return &factory{
		splitFunc:   splitFunc,
		trimFunc:    trimFunc,
		flushPeriod: flushPeriod,
	}
}

// SplitFunc builds a bufio.SplitFunc based on the configuration
func (f *factory) SplitFunc() bufio.SplitFunc {
	return trim.WithFunc(flush.WithPeriod(f.splitFunc, f.flushPeriod), f.trimFunc)
}
