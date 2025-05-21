// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package gopsutilenv

import (
	"context"
	"testing"

	"github.com/shirou/gopsutil/v4/common"
	"github.com/stretchr/testify/assert"
)

func TestRootPaths(t *testing.T) {
	tests := []struct {
		testName string
		rootPath string
		errMsg   string
	}{
		{
			testName: "valid path",
			rootPath: "testdata",
		},
		{
			testName: "empty path",
			rootPath: "",
		},
		{
			testName: "root path",
			rootPath: "/",
		},
		{
			testName: "invalid path",
			rootPath: "invalidpath",
			errMsg:   "invalid root_path: stat invalidpath: no such file or directory",
		},
	}
	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			err := ValidateRootPath(test.rootPath)
			if test.errMsg != "" {
				assert.EqualError(t, err, test.errMsg)
			} else {
				assert.NoError(t, err)
			}
			t.Cleanup(func() { SetGlobalRootPath("") })
		})
	}
}

func TestInconsistentRootPaths(t *testing.T) {
	SetGlobalRootPath("foo")
	err := ValidateRootPath("testdata")
	assert.EqualError(t, err, "inconsistent root_path configuration detected among components: `foo` != `testdata`")

	t.Cleanup(func() { SetGlobalRootPath("") })
}

func TestSetGoPsutilEnvVars(t *testing.T) {
	envMap := SetGoPsutilEnvVars("testdata")
	expectedEnvMap := common.EnvMap{
		common.HostDevEnvKey:  "testdata/dev",
		common.HostEtcEnvKey:  "testdata/etc",
		common.HostRunEnvKey:  "testdata/run",
		common.HostSysEnvKey:  "testdata/sys",
		common.HostVarEnvKey:  "testdata/var",
		common.HostProcEnvKey: "testdata/proc",
	}
	assert.Equal(t, expectedEnvMap, envMap)
	ctx := context.WithValue(context.Background(), common.EnvKey, envMap)
	assert.Equal(t, "testdata/proc", GetEnvWithContext(ctx, string(common.HostProcEnvKey), "default"))
	assert.Equal(t, "testdata/sys", GetEnvWithContext(ctx, string(common.HostSysEnvKey), "default"))
	assert.Equal(t, "testdata/etc", GetEnvWithContext(ctx, string(common.HostEtcEnvKey), "default"))
	assert.Equal(t, "testdata/var", GetEnvWithContext(ctx, string(common.HostVarEnvKey), "default"))
	assert.Equal(t, "testdata/run", GetEnvWithContext(ctx, string(common.HostRunEnvKey), "default"))
	assert.Equal(t, "testdata/dev", GetEnvWithContext(ctx, string(common.HostDevEnvKey), "default"))

	t.Cleanup(func() { SetGlobalRootPath("") })
}

func TestGetEnvWithContext(t *testing.T) {
	envMap := SetGoPsutilEnvVars("testdata")

	ctxEnv := context.WithValue(context.Background(), common.EnvKey, envMap)
	assert.Equal(t, "testdata/proc", GetEnvWithContext(ctxEnv, string(common.HostProcEnvKey), "default"))
	assert.Equal(t, "testdata/sys", GetEnvWithContext(ctxEnv, string(common.HostSysEnvKey), "default"))
	assert.Equal(t, "testdata/etc", GetEnvWithContext(ctxEnv, string(common.HostEtcEnvKey), "default"))
	assert.Equal(t, "testdata/var", GetEnvWithContext(ctxEnv, string(common.HostVarEnvKey), "default"))
	assert.Equal(t, "testdata/run", GetEnvWithContext(ctxEnv, string(common.HostRunEnvKey), "default"))
	assert.Equal(t, "testdata/dev", GetEnvWithContext(ctxEnv, string(common.HostDevEnvKey), "default"))

	assert.Equal(t, "/default", GetEnvWithContext(context.Background(), string(common.HostProcEnvKey), "/default"))
	assert.Equal(t, "/default/foo", GetEnvWithContext(context.Background(), string(common.HostProcEnvKey), "/default", "foo"))
	assert.Equal(t, "/default/foo/bar", GetEnvWithContext(context.Background(), string(common.HostProcEnvKey), "/default", "foo", "bar"))

	t.Cleanup(func() { SetGlobalRootPath("") })
}

func TestSetGoPsutilEnvVarsInvalidRootPath(t *testing.T) {
	err := ValidateRootPath("invalidpath")
	assert.EqualError(t, err, "invalid root_path: stat invalidpath: no such file or directory")

	t.Cleanup(func() { SetGlobalRootPath("") })
}
