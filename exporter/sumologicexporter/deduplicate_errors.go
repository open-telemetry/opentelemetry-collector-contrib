// Copyright 2022 Sumo Logic, Inc.
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

package sumologicexporter

import "fmt"

// deduplicateErrors replaces duplicate instances of the same error in a slice
// with a single error containing the number of times it occurred added as a suffix.
// For example, three occurrences of "error: 502 Bad Gateway"
// are replaced with a single instance of "error: 502 Bad Gateway (x3)".
func deduplicateErrors(errs []error) []error {
	if len(errs) < 2 {
		return errs
	}

	errorsWithCounts := []errorWithCount{}
	for _, err := range errs {
		found := false
		for i := range errorsWithCounts {
			if errorsWithCounts[i].err.Error() == err.Error() {
				found = true
				errorsWithCounts[i].count += 1
				break
			}
		}
		if !found {
			errorsWithCounts = append(errorsWithCounts, errorWithCount{
				err:   err,
				count: 1,
			})
		}
	}

	var uniqueErrors []error
	for _, errorWithCount := range errorsWithCounts {
		if errorWithCount.count == 1 {
			uniqueErrors = append(uniqueErrors, errorWithCount.err)
		} else {
			uniqueErrors = append(uniqueErrors, fmt.Errorf("%s (x%d)", errorWithCount.err, errorWithCount.count))
		}
	}
	return uniqueErrors
}

type errorWithCount struct {
	err   error
	count int
}
