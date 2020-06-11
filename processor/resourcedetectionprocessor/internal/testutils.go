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

package internal

import "go.opentelemetry.io/collector/consumer/pdata"

func NewResource(mp map[string]string) pdata.Resource {
	res := pdata.NewResource()
	res.InitEmpty()
	NewAttributeMap(mp).CopyTo(res.Attributes())
	return res
}

func NewAttributeMap(mp map[string]string) pdata.AttributeMap {
	attr := pdata.NewAttributeMap()
	attr.InitEmptyWithCapacity(len(mp))

	for k, v := range mp {
		attr.Insert(k, pdata.NewAttributeValueString(v))
	}

	return attr
}
