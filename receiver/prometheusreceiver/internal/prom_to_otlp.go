// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal"

import (
	"net"

	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
)

// isDiscernibleHost checks if a host can be used as a value for the 'host.name' key.
// localhost-like hosts and unspecified (0.0.0.0) hosts are not discernible.
func isDiscernibleHost(host string) bool {
	ip := net.ParseIP(host)
	if ip != nil {
		// An IP is discernible if
		//  - it's not local (e.g. belongs to 127.0.0.0/8 or ::1/128) and
		//  - it's not unspecified (e.g. the 0.0.0.0 address).
		return !ip.IsLoopback() && !ip.IsUnspecified()
	}

	if host == "localhost" {
		return false
	}

	// not an IP, not 'localhost', assume it is discernible.
	return true
}

// CreateNodeAndResourcePdata creates the resource data added to OTLP payloads.
func CreateNodeAndResourcePdata(job, instance, scheme string) *pcommon.Resource {
	host, port, err := net.SplitHostPort(instance)
	if err != nil {
		host = instance
	}
	resource := pcommon.NewResource()
	attrs := resource.Attributes()
	attrs.UpsertString(conventions.AttributeServiceName, job)
	if isDiscernibleHost(host) {
		attrs.UpsertString(conventions.AttributeNetHostName, host)
	}
	attrs.UpsertString(conventions.AttributeServiceInstanceID, instance)
	attrs.UpsertString(conventions.AttributeNetHostPort, port)
	attrs.UpsertString(conventions.AttributeHTTPScheme, scheme)

	return &resource
}
