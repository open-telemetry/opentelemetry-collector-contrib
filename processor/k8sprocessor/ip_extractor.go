// Copyright 2020 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package k8sprocessor

import (
	"net"

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
)

type ipExtractor func(attrs pdata.AttributeMap) string

// k8sIPFromAttributes checks if the application, a collector/agent or a prior
// processor has already annotated the batch with IP and if so, uses it
func k8sIPFromAttributes() ipExtractor {
	return func(attrs pdata.AttributeMap) string {
		ip := stringAttributeFromMap(attrs, k8sIPLabelName)
		if ip == "" {
			ip = stringAttributeFromMap(attrs, clientIPLabelName)
		}
		return ip
	}
}

// k8sIPFromHostnameAttributes leverages the observation that most of the metric receivers
// uses "host.name" resource label to identify metrics origin. In k8s environment,
// it's set to a pod IP address. If the value doesn't represent an IP address, we skip it.
func k8sIPFromHostnameAttributes() ipExtractor {
	return func(attrs pdata.AttributeMap) string {
		hostname := stringAttributeFromMap(attrs, conventions.AttributeHostName)
		if net.ParseIP(hostname) != nil {
			return hostname
		}
		return ""
	}
}

func stringAttributeFromMap(attrs pdata.AttributeMap, key string) string {
	if val, ok := attrs.Get(key); ok {
		if val.Type() == pdata.AttributeValueSTRING {
			return val.StringVal()
		}
	}
	return ""
}
