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

package adapter // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"

import (
	// Register parsers and transformers for stanza-based log receivers
	_ "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/output/file"
	_ "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/output/stdout"
	_ "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/csv"
	_ "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/json"
	_ "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/keyvalue"
	_ "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/regex"
	_ "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/severity"
	_ "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/time"
	_ "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/trace"
	_ "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/uri"
	_ "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/add"
	_ "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/copy"
	_ "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/filter"
	_ "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/flatten"
	_ "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/move"
	_ "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/recombine"
	_ "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/remove"
	_ "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/retain"
	_ "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/router"
)
