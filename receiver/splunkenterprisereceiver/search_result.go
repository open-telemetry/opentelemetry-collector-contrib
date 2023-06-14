// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkenterprisereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver"

// metric name and its associated search as a key value pair
var searchDict = map[string]string{ 
    `SplunkLicenseIndexUsageSearch`: `search=search index=_internal source=*license_usage.log type="Usage"| fields idx, b| eval indexname = if(len(idx)=0 OR isnull(idx),"(UNKNOWN)",idx)| stats sum(b) as b by indexname| eval GB=round(b/1024/1024/1024, 3)| fields indexname, GB`,
    }

type searchResponse struct {
    search string
    Jobid  *string `xml:"sid"`
    Return int
    Fields []*Field `xml:"result>field"`
}

type Field struct {
    FieldName string `xml:"k,attr"`
    Value     string `xml:"value>text"`
}

