// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/confmap"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/timeutils"
)

const (
	SortTypeNumeric      = "numeric"
	SortTypeTimestamp    = "timestamp"
	SortTypeAlphabetical = "alphabetical"
)

type SortRule interface {
	validate() error
	sort(re *regexp.Regexp, files []string) ([]string, error)
}

type BaseSortRule struct {
	RegexKey  string `mapstructure:"regex_key,omitempty"`
	Ascending bool   `mapstructure:"ascending,omitempty"`
	SortType  string `mapstructure:"sort_type,omitempty"`
}

type SortRuleImpl struct {
	SortRule
}

func (sr *SortRuleImpl) Unmarshal(component *confmap.Conf) error {
	if !component.IsSet("sort_type") {
		return fmt.Errorf("missing required field 'sort_type'")
	}

	typeInterface := component.Get("sort_type")

	typeString, ok := typeInterface.(string)
	if !ok {
		return fmt.Errorf("non-string type %T for field 'sort_type'", typeInterface)
	}

	switch typeString {
	case SortTypeNumeric:
		var numericSortRule NumericSortRule
		err := component.Unmarshal(&numericSortRule, confmap.WithErrorUnused())
		if err != nil {
			return err
		}
		sr.SortRule = numericSortRule
	case SortTypeAlphabetical:
		var alphabeticalSortRule AlphabeticalSortRule
		err := component.Unmarshal(&alphabeticalSortRule, confmap.WithErrorUnused())
		if err != nil {
			return err
		}
		sr.SortRule = alphabeticalSortRule
	case SortTypeTimestamp:
		var timestampSortRule TimestampSortRule
		err := component.Unmarshal(&timestampSortRule, confmap.WithErrorUnused())
		if err != nil {
			return err
		}
		sr.SortRule = timestampSortRule
	default:
		return fmt.Errorf("invalid sort type %s", typeString)
	}

	return nil
}

type NumericSortRule struct {
	BaseSortRule `mapstructure:",squash"`
}

func (f NumericSortRule) validate() error {
	if f.RegexKey == "" {
		return fmt.Errorf("regex key must be specified for numeric sort")
	}
	return nil
}

type AlphabeticalSortRule struct {
	BaseSortRule `mapstructure:",squash"`
}

func (f AlphabeticalSortRule) validate() error {
	if f.RegexKey == "" {
		return fmt.Errorf("regex key must be specified for alphabetical sort")
	}
	return nil
}

type TimestampSortRule struct {
	BaseSortRule `mapstructure:",squash"`
	Format       string `mapstructure:"format,omitempty"`
	Location     string `mapstructure:"location,omitempty"`
}

func (f TimestampSortRule) validate() error {
	if f.RegexKey == "" {
		return fmt.Errorf("regex key must be specified for timestamp sort")
	}
	if f.Format == "" {
		return fmt.Errorf("format must be specified for timestamp sort")
	}
	if f.Location == "" {
		f.Location = "UTC"
	}
	_, err := time.Parse(f.Format, "")
	if err != nil {
		return fmt.Errorf("error parsing format %s: %w", f.Format, err)
	}

	_, err = time.LoadLocation(f.Location)
	if err != nil {
		return fmt.Errorf("error parsing location %s: %w", f.Location, err)
	}
	return nil
}

func (f NumericSortRule) sort(re *regexp.Regexp, files []string) ([]string, error) {
	var errs error
	sort.Slice(files, func(i, j int) bool {
		valI, valJ, err := extractValues(re, f.RegexKey, files[i], files[j])
		if err != nil {
			errs = multierr.Append(errs, err)
			return false
		}

		numI, err := strconv.Atoi(valI)
		if err != nil {
			errs = multierr.Append(errs, fmt.Errorf("error parsing %s to int: %w", valI, err))
			return false
		}

		numJ, err := strconv.Atoi(valJ)
		if err != nil {
			errs = multierr.Append(errs, fmt.Errorf("error parsing %s to int: %w", valJ, err))
			return false
		}

		if f.Ascending {
			return numI < numJ
		}
		return numI > numJ
	})

	return files, errs
}

func (f TimestampSortRule) sort(re *regexp.Regexp, files []string) ([]string, error) {
	// apply regex to each file and sort the results
	location, err := time.LoadLocation(f.Location)
	if err != nil {
		return files, fmt.Errorf("error loading location %s: %w", f.Location, err)
	}

	var errs error

	sort.Slice(files, func(i, j int) bool {
		valI, valJ, err := extractValues(re, f.RegexKey, files[i], files[j])
		if err != nil {
			errs = multierr.Append(errs, err)
			return false
		}

		timeI, err := timeutils.ParseStrptime(f.Format, valI, location)
		if err != nil {
			errs = multierr.Append(errs, fmt.Errorf("error parsing %s to Time: %w", timeI, err))
			return false
		}

		timeJ, err := timeutils.ParseStrptime(f.Format, valJ, location)
		if err != nil {
			errs = multierr.Append(errs, fmt.Errorf("error parsing %s to Time: %w", timeI, err))
			return false
		}

		// if ascending, return true if timeI is before timeJ
		if f.Ascending {
			return timeI.Before(timeJ)
		}
		return timeI.After(timeJ)
	})

	return files, errs
}

func (f AlphabeticalSortRule) sort(re *regexp.Regexp, files []string) ([]string, error) {
	var errs error
	sort.Slice(files, func(i, j int) bool {
		valI, valJ, err := extractValues(re, f.RegexKey, files[i], files[j])
		if err != nil {
			errs = multierr.Append(errs, err)
			return false
		}

		if f.Ascending {
			return valI < valJ
		}
		return valI > valJ
	})

	return files, errs
}

func extractValues(re *regexp.Regexp, reKey, file1, file2 string) (string, string, error) {
	valI := extractValue(re, reKey, file1)
	if valI == "" {
		return "", "", fmt.Errorf("Unable to find `%s` capture group in regex for file: %s", reKey, file1)
	}
	valJ := extractValue(re, reKey, file2)
	if valJ == "" {
		return "", "", fmt.Errorf("Unable to find `%s` capture group in regex for file: %s", reKey, file2)
	}

	return valI, valJ, nil
}

func extractValue(re *regexp.Regexp, reKey, input string) string {
	match := re.FindStringSubmatch(input)
	if match == nil {
		return ""
	}

	for i, name := range re.SubexpNames() {
		if name == reKey && i < len(match) {
			return match[i]
		}
	}

	return ""
}
