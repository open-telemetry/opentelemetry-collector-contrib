// Copyright 2020, OpenTelemetry Authors
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

package cwlogs // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/cwlogs"

import "github.com/aws/aws-sdk-go/service/cloudwatchlogs"

// EMFSupportedUnits contains the unit collection supported by CloudWatch backend service.
// https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_MetricDatum.html
var EMFSupportedUnits = cloudwatchlogs.StandardUnit_Values()

// IsValidRetentionValue confirms whether the retention is supported with CloudWatchLogs are not and return true if supported
// and vice versa. For more information on log retention, please follow this document retention
// https://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/Working-with-log-groups-and-streams.html#SettingLogRetention
func IsValidRetentionValue(retention int64) bool {
	switch retention {
	case
		0,
		1,
		3,
		5,
		7,
		14,
		30,
		60,
		90,
		120,
		150,
		180,
		365,
		400,
		545,
		731,
		1827,
		2192,
		2557,
		2922,
		3288,
		3653:
		return true
	}
	return false
}
