// Copyright  The OpenTelemetry Authors
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

package attraction // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/attraction"

import (
	"strconv"

	"go.uber.org/zap"

	"go.opentelemetry.io/collector/model/pdata"
)

const (
	stringConversionTarget = "string"
	intConversionTarget    = "int"
	doubleConversionTarget = "double"
)

func convertValue(logger *zap.Logger, key string, to string, v pdata.AttributeValue) {
	switch to {
	case stringConversionTarget:
		switch v.Type() {
		case pdata.AttributeValueTypeString:
		default:
			v.SetStringVal(v.AsString())
		}
	case intConversionTarget:
		switch v.Type() {
		case pdata.AttributeValueTypeInt:
		case pdata.AttributeValueTypeDouble:
			v.SetIntVal(int64(v.DoubleVal()))
		case pdata.AttributeValueTypeBool:
			if v.BoolVal() {
				v.SetIntVal(1)
			} else {
				v.SetIntVal(0)
			}
		case pdata.AttributeValueTypeString:
			s := v.StringVal()
			n, err := strconv.ParseInt(s, 10, 64)
			if err == nil {
				v.SetIntVal(n)
			} else {
				logger.Debug("String could not be converted to int", zap.String("key", key), zap.String("value", s), zap.Error(err))
			}
		default: // leave original value
		}
	case doubleConversionTarget:
		switch v.Type() {
		case pdata.AttributeValueTypeInt:
			v.SetDoubleVal(float64(v.IntVal()))
		case pdata.AttributeValueTypeDouble:
		case pdata.AttributeValueTypeBool:
			if v.BoolVal() {
				v.SetDoubleVal(1)
			} else {
				v.SetDoubleVal(0)
			}
		case pdata.AttributeValueTypeString:
			s := v.StringVal()
			n, err := strconv.ParseFloat(s, 64)
			if err == nil {
				v.SetDoubleVal(n)
			} else {
				logger.Debug("String could not be converted to double", zap.String("key", key), zap.String("value", s), zap.Error(err))
			}
		default: // leave original value
		}
	default: // No-op
	}
}
