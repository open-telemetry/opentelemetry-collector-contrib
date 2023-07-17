// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticbeanstalk

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor/processortest"
)

const xrayConf = "{\"deployment_id\":23,\"version_label\":\"env-version-1234\",\"environment_name\":\"BETA\"}"

type mockFileSystem struct {
	windows  bool
	exists   bool
	path     string
	contents string
}

func (mfs *mockFileSystem) Open(path string) (io.ReadCloser, error) {
	if !mfs.exists {
		return nil, errors.New("file not found")
	}
	mfs.path = path
	f := io.NopCloser(strings.NewReader(mfs.contents))
	return f, nil
}

func (mfs *mockFileSystem) IsWindows() bool {
	return mfs.windows
}

func Test_newDetector(t *testing.T) {
	dcfg := CreateDefaultConfig()
	d, err := NewDetector(processortest.NewNopCreateSettings(), dcfg)

	assert.Nil(t, err)
	assert.NotNil(t, d)
}

func Test_windowsPath(t *testing.T) {
	mfs := &mockFileSystem{windows: true, exists: true, contents: xrayConf}
	d := Detector{fs: mfs}

	r, _, err := d.Detect(context.TODO())

	assert.Nil(t, err)
	assert.NotNil(t, r)
	assert.Equal(t, windowsPath, mfs.path)
}

func Test_fileNotExists(t *testing.T) {
	mfs := &mockFileSystem{exists: false}
	d := Detector{fs: mfs}

	r, _, err := d.Detect(context.TODO())

	assert.Nil(t, err)
	assert.NotNil(t, r)
	assert.Equal(t, 0, r.Attributes().Len())
}

func Test_fileMalformed(t *testing.T) {
	mfs := &mockFileSystem{exists: true, contents: "some overwritten value"}
	d := Detector{fs: mfs}

	r, _, err := d.Detect(context.TODO())

	assert.NotNil(t, err)
	assert.NotNil(t, r)
	assert.Equal(t, 0, r.Attributes().Len())
}

func Test_AttributesDetectedSuccessfully(t *testing.T) {
	mfs := &mockFileSystem{exists: true, contents: xrayConf}
	resourceAttributes := CreateDefaultConfig().ResourceAttributes
	d := Detector{fs: mfs, resourceAttributes: resourceAttributes}

	want := pcommon.NewResource()
	attr := want.Attributes()
	attr.PutStr("cloud.provider", "aws")
	attr.PutStr("cloud.platform", "aws_elastic_beanstalk")
	attr.PutStr("deployment.environment", "BETA")
	attr.PutStr("service.instance.id", "23")
	attr.PutStr("service.version", "env-version-1234")

	r, _, err := d.Detect(context.TODO())

	assert.Nil(t, err)
	assert.NotNil(t, r)
	assert.Equal(t, want.Attributes().AsRaw(), r.Attributes().AsRaw())
}
