package httpheaderextension

import "go.opentelemetry.io/collector/config"

// Config has the configuration for the HTTP Header extension.
type Config struct {
	config.ExtensionSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct
}
