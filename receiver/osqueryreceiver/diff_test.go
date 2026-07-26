// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package osqueryreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func byName(row map[string]string) string {
	return row["name"]
}

func TestDiffRows_NewRow(t *testing.T) {
	previous := []map[string]string{{"name": "curl", "version": "8.0"}}
	current := []map[string]string{
		{"name": "curl", "version": "8.0"},
		{"name": "wget", "version": "1.21"},
	}

	changed := diffRows(byName, previous, current)
	assert.Equal(t, []map[string]string{{"name": "wget", "version": "1.21"}}, changed)
}

func TestDiffRows_ModifiedRow(t *testing.T) {
	previous := []map[string]string{{"name": "curl", "version": "8.0"}}
	current := []map[string]string{{"name": "curl", "version": "8.1"}}

	changed := diffRows(byName, previous, current)
	assert.Equal(t, current, changed)
}

func TestDiffRows_Unchanged(t *testing.T) {
	rows := []map[string]string{{"name": "curl", "version": "8.0"}}

	changed := diffRows(byName, rows, rows)
	assert.Empty(t, changed)
}

func TestDiffRows_EmptyPrevious(t *testing.T) {
	current := []map[string]string{{"name": "curl", "version": "8.0"}}

	changed := diffRows(byName, nil, current)
	assert.Equal(t, current, changed)
}

func TestDiffRows_EmptyCurrent(t *testing.T) {
	previous := []map[string]string{{"name": "curl", "version": "8.0"}}

	changed := diffRows(byName, previous, nil)
	assert.Empty(t, changed)
}
