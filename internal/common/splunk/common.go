// Copyright 2020, OpenTelemetry Authors
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

package splunk

const (
	SFxAccessTokenHeader = "X-Sf-Token"
	SFxAccessTokenLabel  = "com.splunk.signalfx.access_token"
)

type AccessTokenPassthroughConfig struct {
	// Whether to associate datapoints with an organization access token received in request.
	AccessTokenPassthrough bool `mapstructure:"access_token_passthrough"`
}
