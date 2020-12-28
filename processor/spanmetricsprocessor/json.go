// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package spanmetricsprocessor

import "encoding/json"

// JsonSerder wraps the required json Marshal and Unmarshal functions
// to enable mocking in unit tests.
type JsonSerder interface {
	Marshal(v interface{}) ([]byte, error)
	Unmarshal(data []byte, v interface{}) error
}

type JsonSerde struct{}

func (j *JsonSerde) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func (j *JsonSerde) Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
