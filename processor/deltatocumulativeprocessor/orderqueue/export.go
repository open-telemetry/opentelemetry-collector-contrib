// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package orderqueue // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/orderqueue"

func (p *Processor) Export() {
	p.mtx.Lock()
	defer p.mtx.Unlock()

}
