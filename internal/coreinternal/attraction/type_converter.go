// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package attraction // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/attraction"

import (
	"strconv"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
)

const (
	stringConversionTarget = "string"
	intConversionTarget    = "int"
	doubleConversionTarget = "double"
)

func convertValue(logger *zap.Logger, key string, to string, v pcommon.Value) {
	switch to {
	case stringConversionTarget:
		switch v.Type() {
		case pcommon.ValueTypeStr:
		default:
			v.SetStr(v.AsString())
		}
	case intConversionTarget:
		switch v.Type() {
		case pcommon.ValueTypeInt:
		case pcommon.ValueTypeDouble:
			v.SetInt(int64(v.Double()))
		case pcommon.ValueTypeBool:
			if v.Bool() {
				v.SetInt(1)
			} else {
				v.SetInt(0)
			}
		case pcommon.ValueTypeStr:
			s := v.Str()
			n, err := strconv.ParseInt(s, 10, 64)
			if err == nil {
				v.SetInt(n)
			} else {
				logger.Debug("String could not be converted to int", zap.String("key", key), zap.String("value", s), zap.Error(err))
			}
		default:
			logger.Debug("Unable to convert type", zap.String("key", key), zap.String("from", v.Type().String()), zap.String("to", intConversionTarget))
		}
	case doubleConversionTarget:
		switch v.Type() {
		case pcommon.ValueTypeInt:
			v.SetDouble(float64(v.Int()))
		case pcommon.ValueTypeDouble:
		case pcommon.ValueTypeBool:
			if v.Bool() {
				v.SetDouble(1)
			} else {
				v.SetDouble(0)
			}
		case pcommon.ValueTypeStr:
			s := v.Str()
			n, err := strconv.ParseFloat(s, 64)
			if err == nil {
				v.SetDouble(n)
			} else {
				logger.Debug("String could not be converted to double", zap.String("key", key), zap.String("value", s), zap.Error(err))
			}
		default:
			logger.Debug("Unable to convert type", zap.String("key", key), zap.String("from", v.Type().String()), zap.String("to", doubleConversionTarget))
		}
	default: // No-op
	}
}
