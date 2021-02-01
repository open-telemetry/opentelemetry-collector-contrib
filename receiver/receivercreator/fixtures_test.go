// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package receivercreator

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

var pod = observer.Pod{
	UID:       "uid-1",
	Namespace: "default",
	Name:      "pod-1",
	Labels: map[string]string{
		"app":    "redis",
		"region": "west-1",
	},
	Annotations: map[string]string{
		"scrape": "true",
	},
}

var podEndpoint = observer.Endpoint{
	ID:      "pod-1",
	Target:  "localhost",
	Details: &pod,
}

var portEndpoint = observer.Endpoint{
	ID:     "port-1",
	Target: "localhost:1234",
	Details: &observer.Port{
		Name:      "http",
		Pod:       pod,
		Port:      1234,
		Transport: observer.ProtocolTCP,
	},
}

var unsupportedEndpoint = observer.Endpoint{
	ID:      "endpoint-1",
	Target:  "localhost:1234",
	Details: nil,
}
