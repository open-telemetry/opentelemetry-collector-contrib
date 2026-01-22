// Copyright SAP Cloud Infrastructure
// SPDX-License-Identifier: Apache-2.0

package auditdreceiver // import "github.com/cloudoperators/opentelemetry-collector-contrib/receiver/auditdreceiver"

import (
	"go.opentelemetry.io/collector/component"
)

type AuditdReceiverConfig struct {
	Rules []string `mapstructure:"rules,omitempty"`

	_ struct{}
}

func createDefaultConfig() component.Config {
	return &AuditdReceiverConfig{
		Rules: []string{},
	}
}
