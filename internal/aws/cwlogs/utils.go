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

import (
	"fmt"
	"regexp"
)

// Added function to check if value is an accepted number of log retention days
func IsValidRetentionValue(input int64) bool {
	switch input {
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

// Check if the tags input is valid
func ValidateTagsInput(input map[string]*string) error {
	if len(input) > 50 {
		return fmt.Errorf("invalid amount of items. Please input at most 50 tags")
	}
	validKeyPattern := regexp.MustCompile(`^([\p{L}\p{Z}\p{N}_.:/=+\-@]+)$`)
	validValuePattern := regexp.MustCompile(`^([\p{L}\p{Z}\p{N}_.:/=+\-@]*)$`)
	for key, value := range input {
		if len(key) < 1 || len(key) > 128 {
			return fmt.Errorf("key - " + key + " has an invalid length. Please use keys with a length of 1 to 128 characters")
		}
		if len(*value) < 1 || len(*value) > 256 {
			return fmt.Errorf("value - " + *value + " has an invalid length. Please use values with a length of 1 to 256 characters")
		}
		if !validKeyPattern.MatchString(key) {
			return fmt.Errorf("key - " + key + " does not follow the regex pattern" + `^([\p{L}\p{Z}\p{N}_.:/=+\-@]+)$`)
		}
		if !validValuePattern.MatchString(*value) {
			return fmt.Errorf("value - " + *value + " does not follow the regex pattern" + `^([\p{L}\p{Z}\p{N}_.:/=+\-@]*)$`)
		}
	}

	return nil
}
