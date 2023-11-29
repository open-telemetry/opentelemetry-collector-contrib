// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package configschema

import (
	"path/filepath"
	"time"

	"go.opentelemetry.io/collector/config/configtls"
)

type testPerson struct {
	Name string
}

// testStruct comment
type testStruct struct {
	One   string `mapstructure:"one"`
	Two   int    `mapstructure:"two"`
	Three uint   `mapstructure:"three"`
	Four  bool   `mapstructure:"four"`
	// embedded, package qualified comment
	time.Duration `mapstructure:"duration"`
	Squashed      testPerson                 `mapstructure:",squash"`
	PersonPtr     *testPerson                `mapstructure:"person_ptr"`
	PersonStruct  testPerson                 `mapstructure:"person_struct"`
	Persons       []testPerson               `mapstructure:"persons"`
	PersonPtrs    []*testPerson              `mapstructure:"person_ptrs"`
	Ignored       string                     `mapstructure:"-"`
	TLS           configtls.TLSClientSetting `mapstructure:"tls"`
}

func testDR() DirResolver {
	return DirResolver{
		SrcRoot:    filepath.Join("..", ".."),
		ModuleName: DefaultModule,
	}
}
