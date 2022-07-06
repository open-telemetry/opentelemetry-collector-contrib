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

// Skip tests on Windows temporarily, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/11451
//go:build !windows
// +build !windows

package configschema

import (
	"fmt"
	"go/build"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
)

func TestPackageDirLocal(t *testing.T) {
	pkg := resourcetotelemetry.Settings{}
	pkgValue := reflect.ValueOf(pkg)
	dr := testDR(filepath.Join("..", ".."))
	output, err := dr.PackageDir(pkgValue.Type())
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join("..", "..", "pkg", "resourcetotelemetry"), output)
}

func TestPackageDirError(t *testing.T) {
	pkg := pmetric.NewSum()
	pkgType := reflect.ValueOf(pkg).Type()
	srcRoot := "test/fail"
	dr := NewDirResolver(srcRoot, DefaultModule)
	output, err := dr.PackageDir(pkgType)
	assert.Error(t, err)
	assert.Equal(t, "", output)
}

func TestExternalPkgDirErr(t *testing.T) {
	pkg := "random/test"
	pkgPath, err := testDR("../..").externalPackageDir(pkg)
	if assert.Error(t, err) {
		expected := fmt.Sprintf("could not find package: \"%s\"", pkg)
		assert.EqualErrorf(t, err, expected, "")
	}
	assert.Equal(t, pkgPath, "")
}

func TestExternalPkgDir(t *testing.T) {
	dr := testDR("../..")
	testPkg := "github.com/grpc-ecosystem/grpc-gateway/runtime"
	pkgPath, err := dr.externalPackageDir(testPkg)
	assert.NoError(t, err)
	goPath := os.Getenv("GOPATH")
	if goPath == "" {
		goPath = build.Default.GOPATH
	}
	testLine, testVers, err := grepMod(filepath.Join(dr.SrcRoot, "go.mod"), testPkg)
	assert.NoError(t, err)
	expected := fmt.Sprint(filepath.Join(goPath, "pkg", "mod", testLine+"@"+testVers, "runtime"))
	assert.Equal(t, expected, pkgPath)
}

func TestExternalPkgDirReplace(t *testing.T) {
	pkg := path.Join(DefaultModule, "pkg/resourcetotelemetry")
	pkgPath, err := testDR(filepath.Join("..", "..")).externalPackageDir(pkg)
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join("..", "..", "pkg", "resourcetotelemetry"), pkgPath)
}
