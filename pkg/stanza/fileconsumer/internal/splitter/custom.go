// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splitter // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/splitter"

import (
	"bufio"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/flush"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"
)

type customFactory struct {
	splitFunc   bufio.SplitFunc
	flushPeriod time.Duration
}

var _ Factory = (*customFactory)(nil)

func NewCustomFactory(splitFunc bufio.SplitFunc, flushPeriod time.Duration) Factory {
	return &customFactory{
		splitFunc:   splitFunc,
		flushPeriod: flushPeriod,
	}
}

// SplitFunc builds a bufio.SplitFunc based on the configuration
func (f *customFactory) SplitFunc() (bufio.SplitFunc, error) {
	return flush.WithPeriod(f.splitFunc, trim.Nop, f.flushPeriod), nil
}
