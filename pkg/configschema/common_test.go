// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package configschema

import (
	"path/filepath"
	"time"

	"go.opentelemetry.io/collector/config/configtls"
)

const DefaultModule = "github.com/open-telemetry/opentelemetry-collector-contrib"

// testStruct comment
type testStruct struct {
	PersonPtr     *testPerson                `mapstructure:"person_ptr"`
	TLS           configtls.TLSClientSetting `mapstructure:"tls"`
	One           string                     `mapstructure:"one"`
	Squashed      testPerson                 `mapstructure:",squash"`
	PersonStruct  testPerson                 `mapstructure:"person_struct"`
	Ignored       string                     `mapstructure:"-"`
	Persons       []testPerson               `mapstructure:"persons"`
	PersonPtrs    []*testPerson              `mapstructure:"person_ptrs"`
	Two           int                        `mapstructure:"two"`
	Three         uint                       `mapstructure:"three"`
	time.Duration `mapstructure:"duration"`
	Four          bool `mapstructure:"four"`
}

type testPerson struct {
	Name string
}

func testDR() dirResolver {
	return dirResolver{
		srcRoot:    filepath.Join("..", ".."),
		moduleName: DefaultModule,
	}
}
