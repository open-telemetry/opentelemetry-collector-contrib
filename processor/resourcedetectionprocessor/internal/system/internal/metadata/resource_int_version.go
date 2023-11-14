// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/system/internal/metadata"

// SetHostCPUFamilyAsInt sets provided value as "host.cpu.family" attribute as int.
func (rb *ResourceBuilder) SetHostCPUFamilyAsInt(val int64) {
	if rb.config.HostCPUFamily.Enabled {
		rb.res.Attributes().PutInt("host.cpu.family", val)
	}
}

// SetHostCPUModelIDAsInt sets provided value as "host.cpu.model.id" attribute as int.
func (rb *ResourceBuilder) SetHostCPUModelIDAsInt(val int64) {
	if rb.config.HostCPUModelID.Enabled {
		rb.res.Attributes().PutInt("host.cpu.model.id", val)
	}
}
