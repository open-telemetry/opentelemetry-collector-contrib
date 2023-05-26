// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux
// +build linux

package pagingscraper

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const validFile = `Filename				Type		Size		Used		Priority
/dev/dm-2                               partition	67022844	490788		-2
/swapfile                               file		2		1		-3
`

const invalidFile = `INVALID				Type		Size		Used		Priority
/dev/dm-2                               partition	67022844	490788		-2
/swapfile                               file		1048572		0		-3
`

func TestGetPageFileStats_ValidFile(t *testing.T) {
	assert := assert.New(t)
	stats, err := parseSwapsFile(strings.NewReader(validFile))
	assert.NoError(err)

	assert.Equal(*stats[0], pageFileStats{
		deviceName: "/dev/dm-2",
		usedBytes:  502566912,
		freeBytes:  68128825344,
		totalBytes: 68631392256,
	})

	assert.Equal(*stats[1], pageFileStats{
		deviceName: "/swapfile",
		usedBytes:  1024,
		freeBytes:  1024,
		totalBytes: 2048,
	})
}

func TestGetPageFileStats_InvalidFile(t *testing.T) {
	_, err := parseSwapsFile(strings.NewReader(invalidFile))
	assert.Error(t, err)
}

func TestGetPageFileStats_EmptyFile(t *testing.T) {
	_, err := parseSwapsFile(strings.NewReader(""))
	assert.Error(t, err)
}
