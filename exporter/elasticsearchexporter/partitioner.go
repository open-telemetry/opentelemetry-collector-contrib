package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"bytes"
	"context"

	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

type metadataKeysPartitioner struct {
	keys []string
}

func (p metadataKeysPartitioner) GetKey(
	ctx context.Context,
	req exporterhelper.Request,
) string {
	var kb bytes.Buffer
	meta := client.FromContext(ctx).Metadata
	for _, k := range p.keys {
		if len(p.keys) > 0 {
			// Write key as idenitifier if more than one keys
			kb.WriteString(k)
			kb.WriteByte(0)
		}
		if values := meta.Get(k); len(values) != 0 {
			for _, val := range values {
				kb.WriteString(val)
				kb.WriteByte(0)
			}
		}
	}
	return kb.String()
}
