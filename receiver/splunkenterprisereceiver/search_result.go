// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkenterprisereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver"

type searchResponse struct {
    Search string
    Jobid  *string `xml:"sid"`
    Return int
    Fields []*Field `xml:"result>field"`
}

type Field struct {
    FieldName string `xml:"k,attr"`
    Value     string `xml:"value>text"`
}

