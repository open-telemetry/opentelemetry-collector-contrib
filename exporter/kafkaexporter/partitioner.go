// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"

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

func (p metadataKeysPartitioner) MergeCtx(
	ctx1, _ context.Context,
) context.Context {
	m1 := client.FromContext(ctx1).Metadata

	m := make(map[string][]string, len(p.keys))

	for _, key := range p.keys {
		v := m1.Get(key)
		if len(v) == 0 {
			continue
		}
		// We assume that both context's metadata will have the same
		// value since we have defined the custom partitioner using
		// the same keys.
		m[key] = v
	}
	return client.NewContext(
		context.Background(),
		client.Info{Metadata: client.NewMetadata(m)},
	)
}
