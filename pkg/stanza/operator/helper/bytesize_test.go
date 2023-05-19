// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/operatortest"
)

func TestUnmarshalByteSize(t *testing.T) {
	operatortest.ConfigUnmarshalTests{
		DefaultConfig: newHelpersConfig(),
		TestsFile:     filepath.Join(".", "testdata", "bytesize.yaml"),
		Tests: []operatortest.ConfigUnmarshalTest{
			{
				Name: `valid_0`,
				Expect: func() *helpersConfig {
					c := newHelpersConfig()
					c.Size = ByteSize(0)
					return c
				}(),
			},
			{
				Name: `valid_1`,
				Expect: func() *helpersConfig {
					c := newHelpersConfig()
					c.Size = ByteSize(1)
					return c
				}(),
			},
			{
				Name: `valid_3.3`,
				Expect: func() *helpersConfig {
					c := newHelpersConfig()
					c.Size = ByteSize(3)
					return c
				}(),
			},
			{
				Name: `valid_10101010`,
				Expect: func() *helpersConfig {
					c := newHelpersConfig()
					c.Size = ByteSize(10101010)
					return c
				}(),
			},
			{
				Name: `valid_0.01`,
				Expect: func() *helpersConfig {
					c := newHelpersConfig()
					c.Size = ByteSize(0)
					return c
				}(),
			},
			{
				Name: `valid_1kb`,
				Expect: func() *helpersConfig {
					c := newHelpersConfig()
					c.Size = ByteSize(1000)
					return c
				}(),
			},
			{
				Name: `valid_1KB`,
				Expect: func() *helpersConfig {
					c := newHelpersConfig()
					c.Size = ByteSize(1000)
					return c
				}(),
			},
			{
				Name: `valid_1kib`,
				Expect: func() *helpersConfig {
					c := newHelpersConfig()
					c.Size = ByteSize(1024)
					return c
				}(),
			},
			{
				Name: `valid_1KiB`,
				Expect: func() *helpersConfig {
					c := newHelpersConfig()
					c.Size = ByteSize(1024)
					return c
				}(),
			},
			{
				Name: `valid_1mb`,
				Expect: func() *helpersConfig {
					c := newHelpersConfig()
					c.Size = ByteSize(1000 * 1000)
					return c
				}(),
			},
			{
				Name: `valid_1mib`,
				Expect: func() *helpersConfig {
					c := newHelpersConfig()
					c.Size = ByteSize(1024 * 1024)
					return c
				}(),
			},
			{
				Name: `valid_1gb`,
				Expect: func() *helpersConfig {
					c := newHelpersConfig()
					c.Size = ByteSize(1000 * 1000 * 1000)
					return c
				}(),
			},
			{
				Name: `valid_1gib`,
				Expect: func() *helpersConfig {
					c := newHelpersConfig()
					c.Size = ByteSize(1024 * 1024 * 1024)
					return c
				}(),
			},
			{
				Name: `valid_1tb`,
				Expect: func() *helpersConfig {
					c := newHelpersConfig()
					c.Size = ByteSize(1000 * 1000 * 1000 * 1000)
					return c
				}(),
			},
			{
				Name: `valid_1tib`,
				Expect: func() *helpersConfig {
					c := newHelpersConfig()
					c.Size = ByteSize(1024 * 1024 * 1024 * 1024)
					return c
				}(),
			},
			{
				Name: `valid_1pB`,
				Expect: func() *helpersConfig {
					c := newHelpersConfig()
					c.Size = ByteSize(1000 * 1000 * 1000 * 1000 * 1000)
					return c
				}(),
			},
			{
				Name: `valid_1pib`,
				Expect: func() *helpersConfig {
					c := newHelpersConfig()
					c.Size = ByteSize(1024 * 1024 * 1024 * 1024 * 1024)
					return c
				}(),
			},
			{
				Name:      `invalid_3ii3`,
				ExpectErr: true,
			},
			{
				Name:      `invalid_--ii3`,
				ExpectErr: true,
			},
			{
				Name:      `invalid_map`,
				ExpectErr: true,
			},
			{
				Name:      `invalid_map2`,
				ExpectErr: true,
			},
		},
	}.Run(t)
}
