// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearch // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"

// dynamic index attribute key constants
const (
	DataStreamDataset   = "data_stream.dataset"
	DataStreamNamespace = "data_stream.namespace"
	DataStreamType      = "data_stream.type"

	// DocumentIDAttributeName is the attribute name used to specify the document ID.
	DocumentIDAttributeName = "elasticsearch.document_id"

	// DocumentPipelineAttributeName is the attribute name used to specify the document ingest pipeline.
	DocumentPipelineAttributeName = "elasticsearch.ingest_pipeline"
)
