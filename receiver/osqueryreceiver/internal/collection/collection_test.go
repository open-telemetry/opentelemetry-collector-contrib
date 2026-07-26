// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package collection

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	names := []string{
		systemInfoCollectionName,
		packageInfoCollectionName,
		osInfoCollectionName,
		secureBootCollectionName,
		userCollectionName,
	}

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			c, err := New(name)
			require.NoError(t, err)
			assert.Equal(t, name, c.GetName())
			assert.NotEmpty(t, c.GetQuery())
		})
	}
}

func TestNew_UnknownCollection(t *testing.T) {
	c, err := New("does_not_exist")
	require.Error(t, err)
	assert.Nil(t, c)
}

func TestPackageInfoCollection_QueryForOS(t *testing.T) {
	p := packageInfoCollection{}

	assert.Equal(t, packageInfoCollectionQueryHomebrew, p.queryForOS("darwin"))
	assert.Equal(t, packageInfoCollectionQueryHomebrew, p.queryForOS("plan9"))
}

func TestPackageInfoCollection_QueryForOS_Linux(t *testing.T) {
	orig := fileExistsFn
	defer func() { fileExistsFn = orig }()

	p := packageInfoCollection{}

	fileExistsFn = func(path string) bool { return path == "/etc/debian_version" }
	assert.Equal(t, packageInfoCollectionQueryDebian, p.queryForOS("linux"))

	fileExistsFn = func(path string) bool { return path == "/etc/redhat-release" }
	assert.Equal(t, packageInfoCollectionQueryRPM, p.queryForOS("linux"))

	fileExistsFn = func(string) bool { return false }
	assert.Equal(t, packageInfoCollectionQueryDebian, p.queryForOS("linux"))
}

func TestUserCollection_GetQuery(t *testing.T) {
	u := userCollection{}
	assert.Equal(t, userCollectionQueryMap[runtime.GOOS], u.GetQuery())
}

func TestRowKey_Singleton(t *testing.T) {
	row := map[string]string{"hostname": "test-host"}
	for _, c := range []Collection{systemInfoCollection{}, osInfoCollection{}, secureBootCollection{}} {
		assert.Equal(t, singletonRowKey, c.RowKey(row))
	}
}

func TestRowKey_PackageInfo(t *testing.T) {
	p := packageInfoCollection{}
	assert.Equal(t, "curl", p.RowKey(map[string]string{"name": "curl", "version": "8.0"}))
}

func TestRowKey_UserCollection(t *testing.T) {
	u := userCollection{}
	assert.Equal(t, "alice", u.RowKey(map[string]string{"username": "alice"}))
}
