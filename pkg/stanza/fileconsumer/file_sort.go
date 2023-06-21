package fileconsumer

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"time"

	strptime "github.com/observiq/ctimefmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/timeutils"
	"go.uber.org/multierr"
)

const (
	SortTypeNumeric      = "numeric"
	SortTypeTimestamp    = "timestamp"
	SortTypeAlphabetical = "alphabetical"
)

type SortRule struct {
	Regex     string `mapstructure:"regex,omitempty"`
	SortType  string `mapstructure:"sort_type,omitempty"`
	Format    string `mapstructure:"format,omitempty"`
	Location  string `mapstructure:"location,omitempty"`
	Ascending bool   `mapstructure:"ascending,omitempty"`
}

func (f SortRule) validate() error {
	if f.Regex == "" {
		return fmt.Errorf("regex must be specified for sort")
	}

	switch f.SortType {
	case SortTypeNumeric:
		if f.Format != "" {
			return fmt.Errorf("format must be specified for timestamp sort")
		}
		if f.Location != "" {
			return fmt.Errorf("location shouldn't be specified for timestamp sort")
		}
		return nil
	case SortTypeAlphabetical:
		if f.Format != "" {
			return fmt.Errorf("format must be specified for timestamp sort")
		}
		if f.Location != "" {
			return fmt.Errorf("location shouldn't be specified for timestamp sort")
		}
		return nil
	case SortTypeTimestamp:
		if f.Format == "" {
			return fmt.Errorf("format must be specified for timestamp sort")
		}
		if f.Location == "" {
			return fmt.Errorf("location must be specified for timestamp sort")
		}
		_, err := strptime.ToNative(f.Format)
		if err != nil {
			return fmt.Errorf("error parsing format %s: %v", f.Format, err)
		}

		_, err = time.LoadLocation(f.Location)
		if err != nil {
			return fmt.Errorf("error parsing location %s: %v", f.Location, err)
		}
	default:
		return fmt.Errorf("unknown sort type %s", f.SortType)
	}
	return nil
}

func (f SortRule) Sort(files []string) ([]string, error) {
	switch f.SortType {
	case SortTypeNumeric:
		return f.sortInteger(files)
	case SortTypeTimestamp:
		return f.sortTimestamp(files)
	case SortTypeAlphabetical:
		return f.sortAlphabetical(files)
	}
	return files, fmt.Errorf("unknown sort type %s", f.SortType)
}

func (f SortRule) sortInteger(files []string) ([]string, error) {
	re := regexp.MustCompile(f.Regex)

	var errs error
	sort.Slice(files, func(i, j int) bool {
		valI, valJ, err := extractValues(re, files[i], files[j])
		if err != nil {
			errs = multierr.Append(errs, err)
			return false
		}

		numI, err := strconv.Atoi(valI)
		if err != nil {
			errs = multierr.Append(errs, fmt.Errorf("error parsing %s to int: %v", valI, err))
			return false
		}

		numJ, err := strconv.Atoi(valJ)
		if err != nil {
			errs = multierr.Append(errs, fmt.Errorf("error parsing %s to int: %v", valJ, err))
			return false
		}

		if f.Ascending {
			return numI < numJ
		}
		return numI > numJ
	})

	return files, errs
}

func (f SortRule) sortTimestamp(files []string) ([]string, error) {
	// apply regex to each file and sort the results
	re := regexp.MustCompile(f.Regex)
	location, err := time.LoadLocation(f.Location)
	if err != nil {
		return files, fmt.Errorf("error loading location %s: %v", f.Location, err)
	}

	var errs error

	sort.Slice(files, func(i, j int) bool {
		valI, valJ, err := extractValues(re, files[i], files[j])
		if err != nil {
			errs = multierr.Append(errs, err)
			return false
		}

		timeI, err := timeutils.ParseStrptime(f.Format, valI, location)
		if err != nil {
			errs = multierr.Append(errs, fmt.Errorf("error parsing %s to Time: %v", timeI, err))
			return false
		}

		timeJ, err := timeutils.ParseStrptime(f.Format, valJ, location)
		if err != nil {
			errs = multierr.Append(errs, fmt.Errorf("error parsing %s to Time: %v", timeI, err))
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

func (f SortRule) sortAlphabetical(files []string) ([]string, error) {
	re := regexp.MustCompile(f.Regex)

	var errs error
	sort.Slice(files, func(i, j int) bool {
		valI, valJ, err := extractValues(re, files[i], files[j])
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

func extractValues(re *regexp.Regexp, file1, file2 string) (string, string, error) {
	valI := extractValue(re, file1)
	if valI == "" {
		return "", "", fmt.Errorf("Unable to find `value` capture group in regex for file: %s", file1)
	}
	valJ := extractValue(re, file2)
	if valJ == "" {
		return "", "", fmt.Errorf("Unable to find `value` capture group in regex for file: %s", file2)
	}

	return valI, valJ, nil
}

func extractValue(re *regexp.Regexp, input string) string {
	match := re.FindStringSubmatch(input)
	if match == nil {
		return ""
	}

	for i, name := range re.SubexpNames() {
		if name == "value" && i < len(match) {
			return match[i]
		}
	}

	return ""
}
