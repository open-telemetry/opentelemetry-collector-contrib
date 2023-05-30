// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translation // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/translation"

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"
)

const separator = "."

var (
	ErrInvalidFamily  = errors.New("invalid schema family")
	ErrInvalidVersion = errors.New("invalid schema version")
)

// Version is a machine readable version of the string
// schema identifier that can assist in making indexing easier
type Version struct {
	Major, Minor, Patch int
}

// ReadVersionFromPath allows for parsing paths
// that end in a schema version number string
func ReadVersionFromPath(p string) (*Version, error) {
	if p == "" {
		return nil, fmt.Errorf("empty path:%w", ErrInvalidVersion)
	}
	// path.Base doesn't enforce strict forward slashes
	// thus having to test for it here
	if strings.HasSuffix(p, "/") {
		return nil, fmt.Errorf("must not have trailing slash: %w", ErrInvalidVersion)
	}
	return NewVersion(path.Base(p))
}

// GetFamilyAndVersion takes a schemaURL and separates the family from the identifier.
func GetFamilyAndVersion(schemaURL string) (family string, version *Version, err error) {
	u, err := url.Parse(schemaURL)
	if err != nil {
		return "", nil, err
	}
	version, err = ReadVersionFromPath(u.Path)
	if err != nil {
		return "", nil, err
	}

	u.Path = path.Dir(u.Path)
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", nil, fmt.Errorf("must use http(s): %w", ErrInvalidFamily)
	}
	if u.Host == "" {
		return "", nil, fmt.Errorf("must have a host name: %w", ErrInvalidFamily)
	}

	return u.String(), version, err
}

// NewVersion converts a near semver like string (ie 1.4.0) into
// a schema identifier that is comparable for a machine.
// The expected string format can be matched by the following regex:
// [0-9]+\.[0-9]+\.[0-9]+
func NewVersion(s string) (*Version, error) {
	parts := strings.Split(s, separator)

	if l := len(parts); l != 3 {
		return nil, ErrInvalidVersion
	}

	ver := &Version{}

	for i, val := range []*int{&ver.Major, &ver.Minor, &ver.Patch} {
		var err error
		(*val), err = strconv.Atoi(parts[i])
		if err != nil {
			return nil, err
		}
	}

	return ver, nil
}

func (v *Version) String() string {
	return fmt.Sprint(v.Major, separator, v.Minor, separator, v.Patch)
}

// Compare returns a digit to represent if the v is equal, less than,
// or greater than o.
// The values are 0, -1 and 1 respectively.
func (v *Version) Compare(o *Version) int {
	var (
		major = diff(v.Major, o.Major)
		minor = diff(v.Minor, o.Minor)
		patch = diff(v.Patch, o.Patch)
	)
	if major != 0 {
		return major
	}
	if minor != 0 {
		return minor
	}
	return patch
}

func (v *Version) Equal(o *Version) bool {
	return v.Compare(o) == 0
}

func (v *Version) GreaterThan(o *Version) bool {
	return v.Compare(o) > 0
}

func (v *Version) LessThan(o *Version) bool {
	return v.Compare(o) < 0
}

func diff(a, b int) int {
	return gt(a, b) - lt(a, b)
}

func gt(a, b int) int {
	if a > b {
		return 1
	}
	return 0
}

func lt(a, b int) int {
	if a < b {
		return 1
	}
	return 0
}
