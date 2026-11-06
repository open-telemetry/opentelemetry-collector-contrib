// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package addon installs and runs OpAMP addon (sub-)packages alongside the
// Collector.
//
// An addon's behavior is decided by its type, which is carried in the package
// name as "<type>:<name>", the same shape as a Collector confmap URI. The
// Supervisor ships built-in types and lets a distribution register its own
// through Factory, so the OpAMP server never tells the Supervisor how to run
// anything: it can only pick among the types the Supervisor was built with.
package addon

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// typeRegexp is the grammar of an addon type. It is the Collector's component
// type grammar rather than confmap's scheme grammar: confmap schemes follow RFC
// 3986 and do not allow "_", which rules out names like "custom_rules", while
// component types do.
var typeRegexp = regexp.MustCompile(`^[a-zA-Z][0-9a-zA-Z_]{0,62}$`)

// Type is an addon type, the part of a package name before the first ":".
type Type string

// Validate reports whether t is a well-formed addon type.
func (t Type) Validate() error {
	if !typeRegexp.MatchString(string(t)) {
		return fmt.Errorf("invalid addon type %q: must match %s", string(t), typeRegexp.String())
	}
	return nil
}

// ID identifies an addon. Its string form is the OpAMP package name, and is
// also the key under which the addon's configuration is carried in the remote
// config map.
type ID struct {
	Type Type
	// Name is everything after the first ":". It is opaque to the Supervisor
	// and may itself contain ":" or "/".
	Name string
}

// ParseID parses a package name of the form "<type>:<name>".
func ParseID(s string) (ID, error) {
	typ, name, found := strings.Cut(s, ":")
	if !found {
		return ID{}, fmt.Errorf("addon package name %q must have the form \"<type>:<name>\"", s)
	}
	id := ID{Type: Type(typ), Name: name}
	if err := id.Validate(); err != nil {
		return ID{}, fmt.Errorf("addon package name %q: %w", s, err)
	}
	return id, nil
}

// Validate reports whether id is well-formed.
func (id ID) Validate() error {
	if err := id.Type.Validate(); err != nil {
		return err
	}
	if id.Name == "" {
		return errors.New("name after the type must not be empty")
	}
	return nil
}

// String returns the package name for id.
func (id ID) String() string {
	return string(id.Type) + ":" + id.Name
}
