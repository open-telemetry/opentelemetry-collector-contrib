// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchmetricstreamsencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awscloudwatchmetricstreamsencodingextension"

import (
	"encoding/binary"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awscloudwatchmetricstreamsencodingextension/internal/metadata"
)

var errInvalidUvarint = errors.New("invalid OTLP message length: failed to decode varint")

type formatOpenTelemetry10Unmarshaler struct {
	buildInfo component.BuildInfo
}

var _ pmetric.Unmarshaler = (*formatOpenTelemetry10Unmarshaler)(nil)

func (f *formatOpenTelemetry10Unmarshaler) UnmarshalMetrics(record []byte) (pmetric.Metrics, error) {
	md := pmetric.NewMetrics()
	dataLen, pos := len(record), 0
	for pos < dataLen {
		// get size of datum
		nLen, bytesRead := binary.Uvarint(record[pos:])
		if bytesRead <= 0 {
			return md, errInvalidUvarint
		}
		pos += bytesRead

		// unmarshal datum
		req := pmetricotlp.NewExportRequest()
		if err := req.UnmarshalProto(record[pos : pos+int(nLen)]); err != nil {
			return pmetric.Metrics{}, fmt.Errorf("unable to unmarshal input: %w", err)
		}
		pos += int(nLen)

		// add scope name and build info version to
		// the resource metrics
		for i := 0; i < req.Metrics().ResourceMetrics().Len(); i++ {
			rm := req.Metrics().ResourceMetrics().At(i)
			for j := 0; j < rm.ScopeMetrics().Len(); j++ {
				sm := rm.ScopeMetrics().At(j)
				sm.Scope().SetName(metadata.ScopeName)
				sm.Scope().SetVersion(f.buildInfo.Version)
			}
		}
		req.Metrics().ResourceMetrics().MoveAndAppendTo(md.ResourceMetrics())
	}

	if md.DataPointCount() == 0 {
		return pmetric.Metrics{}, errEmptyRecord
	}

	return md, nil
}
