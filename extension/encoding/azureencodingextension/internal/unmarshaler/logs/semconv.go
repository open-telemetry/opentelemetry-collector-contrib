// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler/logs"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler"
)

const attributeErrorMessage = "error.message"

func attrPutErrorMessageBySemConv(attrs pcommon.Map, value string) {
	emitV1 := metadata.ExtensionAzureencodingEmitV1LogConventionsFeatureGate.IsEnabled()
	dontEmitV0 := metadata.ExtensionAzureencodingDontEmitV0LogConventionsFeatureGate.IsEnabled()

	if !dontEmitV0 {
		unmarshaler.AttrPutStrIf(attrs, attributeErrorMessage, value)
	}
	if emitV1 {
		unmarshaler.AttrPutStrIf(attrs, string(conventions.ExceptionMessageKey), value)
	}
}
