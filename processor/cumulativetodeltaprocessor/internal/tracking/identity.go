// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tracking // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor/internal/tracking"

import (
	"bytes"
	"strconv"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
)

type MetricIdentity struct {
	Resource               pcommon.Resource
	InstrumentationLibrary pcommon.InstrumentationScope
	MetricType             pmetric.MetricType
	MetricIsMonotonic      bool
	MetricName             string
	MetricUnit             string
	StartTimestamp         pcommon.Timestamp
	Attributes             pcommon.Map
	MetricValueType        pmetric.NumberDataPointValueType
}

const A = int32('A')
const SEP = byte(0x1E)
const SEPSTR = string(SEP)

func (mi *MetricIdentity) Write(b *bytes.Buffer) {
	b.WriteRune(A + int32(mi.MetricType))
	b.WriteByte(SEP)
	b.WriteRune(A + int32(mi.MetricValueType))
	if mi.Resource.Attributes().Len() > 0 {
		b.WriteByte(SEP)
		resourceHash := pdatautil.MapHash(mi.Resource.Attributes())
		b.Write(resourceHash[:])
	}

	b.WriteByte(SEP)
	b.WriteString(mi.InstrumentationLibrary.Name())
	b.WriteByte(SEP)
	b.WriteString(mi.InstrumentationLibrary.Version())
	b.WriteByte(SEP)
	if mi.MetricIsMonotonic {
		b.WriteByte('Y')
	} else {
		b.WriteByte('N')
	}

	b.WriteByte(SEP)
	b.WriteString(mi.MetricName)
	b.WriteByte(SEP)
	b.WriteString(mi.MetricUnit)

	if mi.Attributes.Len() > 0 {
		b.WriteByte(SEP)
		attrsHash := pdatautil.MapHash(mi.Attributes)
		b.Write(attrsHash[:])
	}
	b.WriteByte(SEP)
	b.WriteString(strconv.FormatInt(int64(mi.StartTimestamp), 36))
}

func (mi *MetricIdentity) IsFloatVal() bool {
	return mi.MetricValueType == pmetric.NumberDataPointValueTypeDouble
}

func (mi *MetricIdentity) IsSupportedMetricType() bool {
	return mi.MetricType == pmetric.MetricTypeSum || mi.MetricType == pmetric.MetricTypeHistogram
}
