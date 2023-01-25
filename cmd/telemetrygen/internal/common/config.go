// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"go.opentelemetry.io/otel/attribute"
)

var (
	errFormatOTLPAttributes       = fmt.Errorf("value should be of the format key=\"value\"")
	errDoubleQuotesOTLPAttributes = fmt.Errorf("value should be a string wrapped in double quotes")
)

type KeyValue map[string]string

var _ pflag.Value = (*KeyValue)(nil)

func (v *KeyValue) String() string {
	return ""
}

func (v *KeyValue) Set(s string) error {
	kv := strings.SplitN(s, "=", 2)
	if len(kv) != 2 {
		return errFormatOTLPAttributes
	}
	val := kv[1]
	if len(val) < 2 || !strings.HasPrefix(val, "\"") || !strings.HasSuffix(val, "\"") {
		return errDoubleQuotesOTLPAttributes
	}

	(*v)[kv[0]] = val[1 : len(val)-1]
	return nil
}

func (v *KeyValue) Type() string {
	return "map[string]string"
}

type Config struct {
	WorkerCount       int
	Rate              int64
	TotalDuration     time.Duration
	ReportingInterval time.Duration

	// OTLP config
	Endpoint           string
	Insecure           bool
	UseHTTP            bool
	Headers            KeyValue
	ResourceAttributes KeyValue
}

func (c *Config) GetAttributes() []attribute.KeyValue {
	var attributes []attribute.KeyValue

	if len(c.ResourceAttributes) > 0 {
		for k, v := range c.ResourceAttributes {
			attributes = append(attributes, attribute.String(k, v))
		}
	}
	return attributes
}
