// Copyright The OpenTelemetry Authors
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

// Package logproto contains files copied from github.com/grafana/loki/pkg/logproto to remove unnecessary dependencies.
// In logproto.pb.go I had to remove few types Query[Request|Response] and SampleQuery[Request|Response] and the gRPC
// service that uses them, because they depend on another loki package stats.
package logproto // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/loki/logproto"
