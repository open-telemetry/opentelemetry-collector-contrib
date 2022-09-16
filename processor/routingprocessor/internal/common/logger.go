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

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/routingprocessor/internal/common"

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/oteltransformationlanguage/ottl"
)

type OTTLLogger struct {
	logger *zap.Logger
}

func NewOTTLLogger(logger *zap.Logger) OTTLLogger {
	return OTTLLogger{
		logger: logger,
	}
}

// WithFields creates a new logger that will include the specified fields
// in all subsequent logs in addition to fields attached to the context
// of the parent logger. Note that fields are not deduplicated.
func (t OTTLLogger) WithFields(fields map[string]any) ottl.Logger {
	newFields := make([]zap.Field, len(fields))
	i := 0

	for k, v := range fields {
		switch val := v.(type) {
		// zap.Any will base64 encode byte slices, but we want them printed as hexadecimal.
		case []byte:
			newFields[i] = zap.String(k, fmt.Sprintf("%x", val))
		default:
			newFields[i] = zap.Any(k, val)
		}
		i++
	}

	return OTTLLogger{
		logger: t.logger.With(newFields...),
	}
}

func (t OTTLLogger) Info(msg string) {
	t.logger.Info(msg)
}

func (t OTTLLogger) Error(msg string) {
	t.logger.Error(msg)
}
