// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"

import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/tokenize"

// Deprecated: [v0.84.0] Use tokenize.FlusherConfig instead
type FlusherConfig = tokenize.FlusherConfig

// Deprecated: [v0.84.0] Use tokenize.NewFlusherConfig instead
var NewFlusherConfig = tokenize.NewFlusherConfig

// Deprecated: [v0.84.0] Use tokenize.Flusher instead
type Flusher = tokenize.Flusher
