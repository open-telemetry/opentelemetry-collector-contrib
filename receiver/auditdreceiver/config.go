package auditdreceiver

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
