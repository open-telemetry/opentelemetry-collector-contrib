// Copyright The OpenTelemetry Authors
// Licensed under the Apache License, Version 2.0
// SPDX-License-Identifier: Apache-2.0

package ottl

// ValueType represents the compile-time type of an OTTL expression.
// This enum provides foundational infrastructure for future improvements
// to the OTTL type system and expression validation.
type ValueType int

const (
	// TypeUnknown indicates that the type of the expression is not known.
	TypeUnknown ValueType = iota

	// TypeString represents string values.
	TypeString

	// TypeInt represents integer values.
	TypeInt

	// TypeFloat represents floating point values.
	TypeFloat

	// TypeBool represents boolean values.
	TypeBool

	// TypeMap represents map-like values.
	TypeMap

	// TypeSlice represents slice or list values.
	TypeSlice
)
