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

package awscloudwatchlogsexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awscloudwatchlogsexporter"

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/cwlogs"
)

func attrsString(attrs map[string]interface{}) map[string]string {
	out := make(map[string]string, len(attrs))
	for k, s := range attrs {
		out[k] = fmt.Sprint(s)
	}
	return out
}

func getLogInfo(rm *cwLogBody, config *Config) (logGroup, logStream string, replaced bool) {
	groupReplaced := true
	streamReplaced := true

	strAttributeMap := attrsString(rm.Resource)

	// Override log group/stream if specified in config. However, in this case, customer won't have correlation experience
	if len(config.LogGroupName) > 0 {
		logGroup, groupReplaced = cwlogs.ReplacePatterns(config.LogGroupName, strAttributeMap, config.logger)
	}
	if len(config.LogStreamName) > 0 {
		logStream, streamReplaced = cwlogs.ReplacePatterns(config.LogStreamName, strAttributeMap, config.logger)
	}
	replaced = groupReplaced && streamReplaced
	return
}
