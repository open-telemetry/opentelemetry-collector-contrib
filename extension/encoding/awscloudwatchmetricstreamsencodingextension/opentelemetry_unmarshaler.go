// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchmetricstreamsencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awscloudwatchmetricstreamsencodingextension"

import (
	"errors"
	"fmt"

	"github.com/gogo/protobuf/proto"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awscloudwatchmetricstreamsencodingextension/internal/metadata"
	expmetrics "github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics"
)

var errInvalidOTLPMessageLength = errors.New("unable to decode data length from message")

type formatOpenTelemetry10Unmarshaler struct {
	buildInfo component.BuildInfo
}

var _ pmetric.Unmarshaler = (*formatOpenTelemetry10Unmarshaler)(nil)

func (f *formatOpenTelemetry10Unmarshaler) UnmarshalMetrics(record []byte) (pmetric.Metrics, error) {
	md := pmetric.NewMetrics()
	dataLen, pos := len(record), 0
	for pos < dataLen {
		// get start of the datum
		n, nLen := proto.DecodeVarint(record)
		if nLen == 0 && n == 0 {
			return md, errInvalidOTLPMessageLength
		}

		// unmarshal datum
		req := pmetricotlp.NewExportRequest()
		pos += nLen
		err := req.UnmarshalProto(record[pos : pos+int(n)])
		pos += int(n)
		if err != nil {
			return pmetric.Metrics{}, fmt.Errorf("unable to unmarshal input: %w", err)
		}

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

		md = expmetrics.Merge(md, req.Metrics())
	}

	if md.DataPointCount() == 0 {
		return pmetric.Metrics{}, errEmptyRecord
	}

	return md, nil
}
