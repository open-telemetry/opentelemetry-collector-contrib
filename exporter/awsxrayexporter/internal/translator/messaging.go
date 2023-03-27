// Copyright 2019, OpenTelemetry Authors
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

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter/internal/translator"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"strings"
)

const MessagingPrefix = "messaging."

func makeMessaging(attributes map[string]pcommon.Value) (map[string]pcommon.Value, map[string]interface{}) {
	var (
		filtered  = make(map[string]pcommon.Value)
		messaging = make(map[string]interface{})
	)

	for key, value := range attributes {
		if strings.HasPrefix(key, MessagingPrefix) {
			messaging[strings.TrimPrefix(key, MessagingPrefix)] = value.AsRaw()
		} else {
			filtered[key] = value
		}
	}

	return filtered, messaging
}
