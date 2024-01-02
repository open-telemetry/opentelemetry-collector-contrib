// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package configschema

import (
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configtls"
)

func TestFieldComments(t *testing.T) {
	v := reflect.ValueOf(testStruct{})
	comments, err := commentsForStruct(v, testDR())
	assert.NoError(t, err)
	assert.Equal(t, "embedded, package qualified comment\n", comments["Duration"])
	assert.Equal(t, "testStruct comment\n", comments["_struct"])
}

func TestExternalType(t *testing.T) {
	u, err := uuid.NewUUID()
	assert.NoError(t, err)
	v := reflect.ValueOf(u)
	comments, err := commentsForStruct(v, testDR())
	assert.NoError(t, err)
	assert.Equal(
		t,
		"A UUID is a 128 bit (16 byte) Universal Unique IDentifier as defined in RFC\n4122.\n",
		comments["_struct"],
	)
}

func TestSubPackage(t *testing.T) {
	s := configtls.TLSClientSetting{}
	v := reflect.ValueOf(s)
	_, err := commentsForStruct(v, testDR())
	require.NoError(t, err)
}
