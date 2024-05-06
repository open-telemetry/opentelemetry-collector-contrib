// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package reorderprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/reorderprocessor"

func (p *Processor) Export() {
	p.mtx.Lock()
	defer p.mtx.Unlock()

}
