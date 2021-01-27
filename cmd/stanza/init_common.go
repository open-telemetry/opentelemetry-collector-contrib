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

package main

import (
	// Load packages when importing input operators
	_ "github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/input/file"
	_ "github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/input/forward"
	_ "github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/input/generate"
	_ "github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/input/k8sevent"
	_ "github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/input/stanza"
	_ "github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/input/stdin"
	_ "github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/input/tcp"
	_ "github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/input/udp"

	_ "github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/parser/json"
	_ "github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/parser/regex"
	_ "github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/parser/severity"
	_ "github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/parser/syslog"
	_ "github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/parser/time"

	_ "github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/transformer/filter"
	_ "github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/transformer/hostmetadata"
	_ "github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/transformer/k8smetadata"
	_ "github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/transformer/metadata"
	_ "github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/transformer/noop"
	_ "github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/transformer/ratelimit"
	_ "github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/transformer/recombine"
	_ "github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/transformer/restructure"
	_ "github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/transformer/router"

	_ "github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/output/drop"
	_ "github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/output/elastic"
	_ "github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/output/file"
	_ "github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/output/forward"
	_ "github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/output/googlecloud"
	_ "github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/output/newrelic"
	_ "github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/output/otlp"
	_ "github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/output/stdout"
)
