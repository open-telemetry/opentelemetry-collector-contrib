// Copyright  The OpenTelemetry Authors
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

package schema // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/schema"

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
	ErrInvalidIdentifier = errors.New("invalid schema version")
	ErrInvalidFamily     = errors.New("invalid schema family")
)

// Version is a machine readable version of the string
// schema identifier that can assist in making indexing easier
type Version struct {
	Major, Minor, Patch int
}

// ReadVersionFromPath allows for parsing paths
// that end in a schema indentifier
func ReadVersionFromPath(p string) (*Version, error) {
	if p == "" {
		return nil, fmt.Errorf("empty path:%w", ErrInvalidIdentifier)
	}
	// path.Base doesn't enforce strict forward slashes
	// thus having to test for it here
	if strings.HasSuffix(p, "/") {
		return nil, fmt.Errorf("must not have trailing slash: %w", ErrInvalidIdentifier)
	}
	ident := path.Base(p)
	return NewVersion(ident)
}

// GetFamilyAndVersion takes a schemaURL and separates the family from the identifier.
func GetFamilyAndVersion(schemaURL string) (family string, id *Version, err error) {
	u, err := url.Parse(schemaURL)
	if err != nil {
		return "", nil, err
	}
	id, err = ReadVersionFromPath(u.Path)
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

	return u.String(), id, err
}

// NewVersion converts a near semver like string (ie 1.4.0) into
// a schema identifier that is comparable for a machine.
// The expected string format can be matched by the following regex:
// [0-9]+\.[0-9]+\.[0-9]+
func NewVersion(s string) (*Version, error) {
	parts := strings.Split(s, separator)

	if l := len(parts); l != 3 {
		return nil, ErrInvalidIdentifier
	}

	vals := make([]int, len(parts))
	for i, val := range parts {
		var err error

		vals[i], err = strconv.Atoi(val)
		if err != nil {
			return nil, err
		}
	}

	return &Version{
		Major: vals[0],
		Minor: vals[1],
		Patch: vals[2],
	}, nil
}

func (v *Version) String() string {
	return fmt.Sprint(v.Major, separator, v.Minor, separator, v.Patch)
}

// Compare returns a digit to represent if the v is equal, less than,
// or greater than o.
// The values are 0, 1 and -1 respectively.
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

func (id *Version) Equal(v *Version) bool {
	return id.Compare(v) == 0
}

func (id *Version) GreaterThan(v *Version) bool {
	return id.Compare(v) > 0
}

func (id *Version) LessThan(v *Version) bool {
	return id.Compare(v) < 0
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
