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

package ec2

type Config struct {
	// TagsToAdd is a list of ec2 instance tags that users want
	// to add as resource attributes to processed data
	TagsToAdd []string `mapstructure:"tags_to_add"`
	// AddAllTags is a boolean flag that adds all detected tags
	// as resource attributes, and overrides the list TagsToAdd
	AddAllTags bool `mapstructure:"add_all_tags"`
}
