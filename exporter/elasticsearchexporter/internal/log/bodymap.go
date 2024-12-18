// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package log // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/ecs/internal/log"

import (
	"errors"
	"fmt"

	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

var ErrInvalidTypeForBodyMapMode = errors.New("invalid log record body type for 'bodymap' mapping mode")

type bodyMapEncoder struct{}

func (e *bodyMapEncoder) EncodeLog(resource pcommon.Resource, resourceSchemaURL string, record plog.LogRecord, scope pcommon.InstrumentationScope, scopeSchemaURL string) ([]byte, error) {
	body := record.Body()
	if body.Type() != pcommon.ValueTypeMap {
		return nil, fmt.Errorf("%w: %q", ErrInvalidTypeForBodyMapMode, body.Type())
	}

	return jsoniter.Marshal(body.Map().AsRaw())
}
