// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/system/internal/metadata"

// SetHostCPUSteppingAsInt sets provided value as "host.cpu.stepping" attribute as int.
func (rb *ResourceBuilder) SetHostCPUSteppingAsInt(val int64) {
	if rb.config.HostCPUModelID.Enabled {
		rb.res.Attributes().PutInt("host.cpu.stepping", val)
	}
}
