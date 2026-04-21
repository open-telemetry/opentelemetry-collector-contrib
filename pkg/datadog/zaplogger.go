// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadog // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog"

import (
	tracelog "github.com/DataDog/datadog-agent/pkg/trace/log"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/agentcomponents"
)

var _ tracelog.Logger = &Zaplogger{}

// Zaplogger is an alias for agentcomponents.Zaplogger for backwards compatibility
//
// Deprecated [v0.129.0]: This type has been deprecated, use pkg/datadog/agentcomponents.ZapLogger instead
type Zaplogger = agentcomponents.ZapLogger
