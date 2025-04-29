// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearch // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"

import "fmt"

type Index struct {
	Index     string
	Type      string
	Dataset   string
	Namespace string
}

func NewDataStreamIndex(typ, dataset, namespace string) Index {
	return Index{
		Index:     fmt.Sprintf("%s-%s-%s", typ, dataset, namespace),
		Type:      typ,
		Dataset:   dataset,
		Namespace: namespace,
	}
}

func (i Index) IsDataStream() bool {
	return i.Type != "" && i.Dataset != "" && i.Namespace != ""
}
