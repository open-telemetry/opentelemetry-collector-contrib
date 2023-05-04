// Copyright The OpenTelemetry Authors
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

// Package cloudfoundryreceiver implements a receiver that can be used by the
// Opentelemetry collector to receive Cloud Foundry metrics via its Reverse
// Log Proxy (RLP) Gateway component. The protocol is handled by the
// go-loggregator library, which uses HTTP to connect to the gateway and receive
// JSON-protobuf encoded v2 Envelope messages as documented by loggregator-api.
package cloudfoundryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudfoundryreceiver"
