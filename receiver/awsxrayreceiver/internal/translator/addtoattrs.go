// Copyright The OpenTelemetry Authors
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

package translator

import (
	"go.opentelemetry.io/collector/consumer/pdata"
)

const (
	// AWSXRayInProgressAttribute is the `in_progress` flag in an X-Ray segment
	AWSXRayInProgressAttribute = "aws.xray.inprogress"

	// AWSXRayXForwardedForAttribute is the `x_forwarded_for` flag in an X-Ray segment
	AWSXRayXForwardedForAttribute = "aws.xray.x_forwarded_for"

	// AWSXRayResourceARNAttribute is the `resource_arn` field in an X-Ray segment
	AWSXRayResourceARNAttribute = "aws.xray.resource_arn"

	// AWSXRayTracedAttribute is the `traced` field in an X-Ray subsegment
	AWSXRayTracedAttribute = "aws.xray.traced"
)

func addBool(val *bool, attrKey string, attrs *pdata.AttributeMap) {
	if val != nil {
		attrs.UpsertBool(attrKey, *val)
	}
}

func addString(val *string, attrKey string, attrs *pdata.AttributeMap) {
	if val != nil {
		attrs.UpsertString(attrKey, *val)
	}
}

func addInt(val *int, attrKey string, attrs *pdata.AttributeMap) {
	if val != nil {
		attrs.UpsertInt(attrKey, int64(*val))
	}
}
