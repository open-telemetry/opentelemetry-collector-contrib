// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package testutils // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/model/internal/testutils"

import "go.opentelemetry.io/collector/model/pdata"

func fillAttributeMap(attrs pdata.AttributeMap, mp map[string]string) {
	attrs.Clear()
	attrs.EnsureCapacity(len(mp))
	for k, v := range mp {
		attrs.Insert(k, pdata.NewAttributeValueString(v))
	}
}

// NewAttributeMap creates a new attribute map (string only)
// from a Go map
func NewAttributeMap(mp map[string]string) pdata.AttributeMap {
	attrs := pdata.NewAttributeMap()
	fillAttributeMap(attrs, mp)
	return attrs
}
