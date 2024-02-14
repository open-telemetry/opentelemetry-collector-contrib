// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cwlogs // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/cwlogs"

import (
	"errors"
	"fmt"
	"regexp"
)

// Added function to check if value is an accepted number of log retention days
func ValidateRetentionValue(input int64) error {
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
		return nil
	}
	return errors.New("invalid value for retention policy.  Please make sure to use the following values: 0 (Never Expire), 1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731, 1827, 2192, 2557, 2922, 3288, or 3653")
}

// Check if the tags input is valid
func ValidateTagsInput(input map[string]*string) error {
	if input != nil && len(input) < 1 {
		return fmt.Errorf("invalid amount of items. Please input at least 1 tag or remove the tag field")
	}
	if len(input) > 50 {
		return fmt.Errorf("invalid amount of items. Please input at most 50 tags")
	}
	// The regex for the Key and Value requires "alphanumerics, whitespace, and _.:/=+-!" as noted here: https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_CreateLogGroup.html#:~:text=%5E(%5B%5Cp%7BL%7D%5Cp%7BZ%7D%5Cp%7BN%7D_.%3A/%3D%2B%5C%2D%40%5D%2B)%24
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
