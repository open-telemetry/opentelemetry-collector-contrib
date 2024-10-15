// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseDarwinDescription(t *testing.T) {
	testCases := []struct {
		desc     string
		input    string
		expected string
	}{
		{
			desc:     "basic use case",
			input:    "ProductName: macOS\nProductVersion: 15.0.1\nBuildVersion: 24A348",
			expected: "macOS 15.0.1",
		},
		{
			desc:     "excessive white space",
			input:    "			   \n	ProductName:		 macOS\nProductVersion: 15.0.1     \n \n	BuildVersion: 24A348\n",
			expected: "macOS 15.0.1",
		},
		{
			desc:     "random ordering & excessive white space",
			input:    "\nProductVersion: 15.0.1     \n \n	BuildVersion: 24A348\n			   \n	ProductName:		 macOS",
			expected: "macOS 15.0.1",
		},
		{
			desc:     "blank input",
			input:    "",
			expected: "",
		},
		{
			desc:     "missing ProductVersion",
			input:    "ProductName: macOS\nBuildVersion: 24A348",
			expected: "",
		},
		{
			desc:     "missing ProductName",
			input:    "ProductVersion: 15.0.1\nBuildVersion: 24A348",
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			actual := ParseDarwinDescription(tc.input)
			require.Equal(t, tc.expected, actual)
		})
	}
}

func TestParseLinuxDescription(t *testing.T) {
	testCases := []struct {
		desc     string
		input    string
		expected string
	}{
		{
			desc:     "basic use case",
			input:    "Description: Ubuntu 20.04.6 LTS",
			expected: "Ubuntu 20.04.6 LTS",
		},
		{
			desc:     "excessive white space",
			input:    "         \n Description:            	Ubuntu 20.04.6 LTS			\n		",
			expected: "Ubuntu 20.04.6 LTS",
		},
		{
			desc:     "blank input",
			input:    "",
			expected: "",
		},
		{
			desc:     "missing Description",
			input:    "Foo: Ubuntu 20.04.6 LTS",
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			actual := ParseLinuxDescription(tc.input)
			require.Equal(t, tc.expected, actual)
		})
	}
}

func TestParseWindowsDescription(t *testing.T) {
	testCases := []struct {
		desc     string
		input    string
		expected string
	}{
		{
			desc:     "basic use case",
			input:    "Microsoft Windows [Version 10.0.20348.2700]",
			expected: "Microsoft Windows [Version 10.0.20348.2700]",
		},
		{
			desc:     "excessive surrounding white space",
			input:    "         \n Microsoft Windows [Version 10.0.20348.2700]			\n		",
			expected: "Microsoft Windows [Version 10.0.20348.2700]",
		},
		{
			desc:     "excessive white space",
			input:    "         \n Microsoft       Windows [Version 10.0.20348.2700]			\n		",
			expected: "Microsoft       Windows [Version 10.0.20348.2700]",
		},
		{
			desc:     "blank input",
			input:    "",
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			actual := ParseWindowsDescription(tc.input)
			require.Equal(t, tc.expected, actual)
		})
	}
}
