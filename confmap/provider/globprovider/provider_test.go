// Copyright The OpenTelemetry Authors
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

package globprovider

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/collector/confmap"
)

func TestValidateProviderScheme(t *testing.T) {
	assert.NoError(t, ValidateProviderScheme(NewWithSettings(confmap.ProviderSettings{})))
}

func TestEmptyName(t *testing.T) {
	fp := NewWithSettings(confmap.ProviderSettings{})
	_, err := fp.Retrieve(context.Background(), "", nil)
	require.Error(t, err)
	require.NoError(t, fp.Shutdown(context.Background()))
}

func TestUnsupportedScheme(t *testing.T) {
	fp := NewWithSettings(confmap.ProviderSettings{})
	_, err := fp.Retrieve(context.Background(), "https://", nil)
	assert.Error(t, err)
	assert.NoError(t, fp.Shutdown(context.Background()))
}

func TestNonExistent(t *testing.T) {
	fp := NewWithSettings(confmap.ProviderSettings{})
	expectedMap := confmap.New()

	ret, err := fp.Retrieve(context.Background(), schemePrefix+filepath.Join("testdata", "non-existent/*.yaml"), nil)
	assert.NoError(t, err)
	retMap, err := ret.AsConf()
	assert.NoError(t, err)
	assert.Equal(t, expectedMap, retMap)

	ret, err = fp.Retrieve(context.Background(), schemePrefix+absolutePath(t, filepath.Join("testdata", "non-existent/*.yaml")), nil)
	assert.NoError(t, err)
	retMap, err = ret.AsConf()
	assert.NoError(t, err)
	assert.Equal(t, expectedMap, retMap)

	require.NoError(t, fp.Shutdown(context.Background()))
}

func TestInvalidYAML(t *testing.T) {
	fp := NewWithSettings(confmap.ProviderSettings{})
	_, err := fp.Retrieve(context.Background(), schemePrefix+filepath.Join("testdata", "invalid-yaml.*"), nil)
	assert.Error(t, err)
	_, err = fp.Retrieve(context.Background(), schemePrefix+absolutePath(t, filepath.Join("testdata", "invalid-yaml.*")), nil)
	assert.Error(t, err)
	require.NoError(t, fp.Shutdown(context.Background()))
}

func TestInvalidPattern(t *testing.T) {
	fp := NewWithSettings(confmap.ProviderSettings{})
	pattern := "["
	_, err := fp.Retrieve(context.Background(), schemePrefix+pattern, nil)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "syntax error in pattern")
	_, err = fp.Retrieve(context.Background(), schemePrefix+pattern, nil)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "syntax error in pattern")
	require.NoError(t, fp.Shutdown(context.Background()))
}

func TestRelativePath(t *testing.T) {
	fp := NewWithSettings(confmap.ProviderSettings{})
	ret, err := fp.Retrieve(context.Background(), schemePrefix+filepath.Join("testdata", "multiple", "*.yaml"), nil)
	require.NoError(t, err)
	retMap, err := ret.AsConf()
	assert.NoError(t, err)
	expectedMap := confmap.NewFromStringMap(map[string]interface{}{
		"processors::batch/first":          nil,
		"processors::batch/second":         nil,
		"exporters::otlp/first::endpoint":  "localhost:4317",
		"exporters::otlp/second::endpoint": "localhost:4318",
	})
	assert.Equal(t, expectedMap, retMap)
	assert.NoError(t, fp.Shutdown(context.Background()))
}

func TestAbsolutePath(t *testing.T) {
	fp := NewWithSettings(confmap.ProviderSettings{})
	ret, err := fp.Retrieve(context.Background(), schemePrefix+absolutePath(t, filepath.Join("testdata", "multiple", "*.yaml")), nil)
	require.NoError(t, err)
	retMap, err := ret.AsConf()
	assert.NoError(t, err)
	expectedMap := confmap.NewFromStringMap(map[string]interface{}{
		"processors::batch/first":          nil,
		"processors::batch/second":         nil,
		"exporters::otlp/first::endpoint":  "localhost:4317",
		"exporters::otlp/second::endpoint": "localhost:4318",
	})
	assert.Equal(t, expectedMap, retMap)
	assert.NoError(t, fp.Shutdown(context.Background()))
}

func TestMergeOrder(t *testing.T) {
	fp := NewWithSettings(confmap.ProviderSettings{})
	ret, err := fp.Retrieve(context.Background(), schemePrefix+absolutePath(t, filepath.Join("testdata", "ordered", "*.yaml")), nil)
	require.NoError(t, err)
	retMap, err := ret.AsConf()
	assert.NoError(t, err)
	expectedMap := confmap.NewFromStringMap(map[string]interface{}{
		"exporters::otlp::endpoint": "localhost:4319",
	})
	assert.Equal(t, expectedMap, retMap)
	assert.NoError(t, fp.Shutdown(context.Background()))
}

func absolutePath(t *testing.T, relativePath string) string {
	dir, err := os.Getwd()
	require.NoError(t, err)
	return filepath.Join(dir, relativePath)
}

// TODO: Replace this with the upstream exporter version after we upgrade to v0.58.0
var schemeValidator = regexp.MustCompile("^[A-Za-z][A-Za-z0-9+.-]+$")

// ValidateProviderScheme enforces that given confmap.Provider.Scheme() object is following the restriction defined by the collector:
//   - Checks that the scheme name follows the restrictions defined https://datatracker.ietf.org/doc/html/rfc3986#section-3.1
//   - Checks that the scheme name has at leas two characters per the confmap.Provider.Scheme() comment.
func ValidateProviderScheme(p confmap.Provider) error {
	scheme := p.Scheme()
	if len(scheme) < 2 {
		return fmt.Errorf("scheme must be at least 2 characters long: %q", scheme)
	}

	if !schemeValidator.MatchString(scheme) {
		return fmt.Errorf(
			`scheme names consist of a sequence of characters beginning with a letter and followed by any combination of
			letters, digits, \"+\", \".\", or \"-\": %q`,
			scheme,
		)
	}

	return nil
}
