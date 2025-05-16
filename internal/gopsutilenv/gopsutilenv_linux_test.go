// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package gopsutilenv

import (
	"context"
	"testing"

	"github.com/shirou/gopsutil/v4/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConsistentRootPaths(t *testing.T) {
	assert.NoError(t, testValidate("testdata"))
	assert.NoError(t, testValidate(""))
	assert.NoError(t, testValidate("/"))
}

func TestInconsistentRootPaths(t *testing.T) {
	SetGlobalRootPath("foo")
	err := testValidate("testdata")
	assert.EqualError(t, err, "inconsistent root_path configuration detected among components: `foo` != `testdata`")

	SetGlobalRootPath("")
}

func TestSetGoPsutilEnvVars(t *testing.T) {
	envMap, err := SetGoPsutilEnvVars("testdata")
	require.NoError(t, err)
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

	SetGlobalRootPath("")
}

func TestSetGoPsutilEnvVarsInconsistentRootPaths(t *testing.T) {
	SetGlobalRootPath("foo")
	_, err := SetGoPsutilEnvVars("testdata")
	assert.EqualError(t, err, "inconsistent root_path configuration detected among components: `foo` != `testdata`")

	SetGlobalRootPath("")
}

func TestGetEnvWithContext(t *testing.T) {
	envMap, err := SetGoPsutilEnvVars("testdata")
	require.NoError(t, err)
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

	SetGlobalRootPath("")
}

func TestSetGoPsutilEnvVarsInvalidRootPath(t *testing.T) {
	_, err := SetGoPsutilEnvVars("invalidpath")
	assert.EqualError(t, err, "invalid root_path: stat invalidpath: no such file or directory")

	SetGlobalRootPath("")
}

func testValidate(rootPath string) error {
	err := ValidateRootPath(rootPath)
	globalRootPath = ""
	return err
}
