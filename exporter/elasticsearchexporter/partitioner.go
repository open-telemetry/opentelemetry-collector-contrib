// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"bytes"
	"context"

	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/exporter/exporterhelper/xexporterhelper"
)

type metadataKeysPartitioner struct {
	keys []string
}

func (p metadataKeysPartitioner) GetKey(
	ctx context.Context,
	_ xexporterhelper.Request,
) string {
	var kb bytes.Buffer
	meta := client.FromContext(ctx).Metadata

	var afterFirst bool
	for _, k := range p.keys {
		if values := meta.Get(k); len(values) != 0 {
			if afterFirst {
				kb.WriteByte(0)
			}
			kb.WriteString(k)
			afterFirst = true
			for _, val := range values {
				kb.WriteByte(0)
				kb.WriteString(val)
			}
		}
	}
	return kb.String()
}
