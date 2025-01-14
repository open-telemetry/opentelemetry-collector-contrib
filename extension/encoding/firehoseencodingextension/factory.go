package firehoseencodingextension

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/cloudwatchencodingextension/internal/metadata"
	"go.opentelemetry.io/collector/extension"
)

func NewFactory() extension.Factory {
	return extension.NewFactory(
		metadata.Type,
		createDefaultConfig,
		createExtension,
		metadata.ExtensionStability,
	)
}
