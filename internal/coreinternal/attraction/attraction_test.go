// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package attraction

import (
	"context"
	"crypto/sha1" // #nosec
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

// Common structure for all the Tests
type testCase struct {
	name               string
	inputAttributes    map[string]any
	expectedAttributes map[string]any
}

// runIndividualTestCase is the common logic of passing trace data through a configured attributes processor.
func runIndividualTestCase(t *testing.T, tt testCase, ap *AttrProc) {
	t.Run(tt.name, func(t *testing.T) {
		inputMap := pcommon.NewMap()
		assert.NoError(t, inputMap.FromRaw(tt.inputAttributes))
		ap.Process(context.TODO(), nil, inputMap)
		require.Equal(t, tt.expectedAttributes, inputMap.AsRaw())
	})
}

func TestAttributes_InsertValue(t *testing.T) {
	testCases := []testCase{
		// Ensure `attribute1` is set for spans with no attributes.
		{
			name:            "InsertEmptyAttributes",
			inputAttributes: map[string]any{},
			expectedAttributes: map[string]any{
				"attribute1": int64(123),
			},
		},
		// Ensure `attribute1` is set.
		{
			name: "InsertKeyNoExists",
			inputAttributes: map[string]any{
				"anotherkey": "bob",
			},
			expectedAttributes: map[string]any{
				"anotherkey": "bob",
				"attribute1": int64(123),
			},
		},
		// Ensures no insert is performed because the keys `attribute1` already exists.
		{
			name: "InsertKeyExists",
			inputAttributes: map[string]any{
				"attribute1": "bob",
			},
			expectedAttributes: map[string]any{
				"attribute1": "bob",
			},
		},
	}

	cfg := &Settings{
		Actions: []ActionKeyValue{
			{Key: "attribute1", Action: INSERT, Value: 123},
		},
	}

	ap, err := NewAttrProc(cfg)
	require.Nil(t, err)
	require.NotNil(t, ap)

	for _, tt := range testCases {
		runIndividualTestCase(t, tt, ap)
	}
}

func TestAttributes_InsertFromAttribute(t *testing.T) {

	testCases := []testCase{
		// Ensure no attribute is inserted because because attributes do not exist.
		{
			name:               "InsertEmptyAttributes",
			inputAttributes:    map[string]any{},
			expectedAttributes: map[string]any{},
		},
		// Ensure no attribute is inserted because because from_attribute `string_key` does not exist.
		{
			name: "InsertMissingFromAttribute",
			inputAttributes: map[string]any{
				"bob": int64(1),
			},
			expectedAttributes: map[string]any{
				"bob": int64(1),
			},
		},
		// Ensure `string key` is set.
		{
			name: "InsertAttributeExists",
			inputAttributes: map[string]any{
				"anotherkey": int64(8892342),
			},
			expectedAttributes: map[string]any{
				"anotherkey": int64(8892342),
				"string key": int64(8892342),
			},
		},
		// Ensures no insert is performed because the keys `string key` already exist.
		{
			name: "InsertKeysExists",
			inputAttributes: map[string]any{
				"anotherkey": int64(8892342),
				"string key": "here",
			},
			expectedAttributes: map[string]any{
				"anotherkey": int64(8892342),
				"string key": "here",
			},
		},
	}
	cfg := &Settings{
		Actions: []ActionKeyValue{
			{Key: "string key", Action: INSERT, FromAttribute: "anotherkey"},
		},
	}

	ap, err := NewAttrProc(cfg)
	require.Nil(t, err)
	require.NotNil(t, ap)

	for _, tt := range testCases {
		runIndividualTestCase(t, tt, ap)
	}
}

func TestAttributes_UpdateValue(t *testing.T) {

	testCases := []testCase{
		// Ensure no changes to the span as there is no attributes map.
		{
			name:               "UpdateNoAttributes",
			inputAttributes:    map[string]any{},
			expectedAttributes: map[string]any{},
		},
		// Ensure no changes to the span as the key does not exist.
		{
			name: "UpdateKeyNoExist",
			inputAttributes: map[string]any{
				"boo": "foo",
			},
			expectedAttributes: map[string]any{
				"boo": "foo",
			},
		},
		// Ensure the attribute `db.secret` is updated.
		{
			name: "UpdateAttributes",
			inputAttributes: map[string]any{
				"db.secret": "password1234",
			},
			expectedAttributes: map[string]any{
				"db.secret": "redacted",
			},
		},
	}

	cfg := &Settings{
		Actions: []ActionKeyValue{
			{Key: "db.secret", Action: UPDATE, Value: "redacted"},
		},
	}

	ap, err := NewAttrProc(cfg)
	require.Nil(t, err)
	require.NotNil(t, ap)

	for _, tt := range testCases {
		runIndividualTestCase(t, tt, ap)
	}
}

func TestAttributes_UpdateFromAttribute(t *testing.T) {

	testCases := []testCase{
		// Ensure no changes to the span as there is no attributes map.
		{
			name:               "UpdateNoAttributes",
			inputAttributes:    map[string]any{},
			expectedAttributes: map[string]any{},
		},
		// Ensure the attribute `boo` isn't updated because attribute `foo` isn't present in the span.
		{
			name: "UpdateKeyNoExistFromAttribute",
			inputAttributes: map[string]any{
				"boo": "bob",
			},
			expectedAttributes: map[string]any{
				"boo": "bob",
			},
		},
		// Ensure no updates as the target key `boo` doesn't exists.
		{
			name: "UpdateKeyNoExistMainAttributed",
			inputAttributes: map[string]any{
				"foo": "over there",
			},
			expectedAttributes: map[string]any{
				"foo": "over there",
			},
		},
		// Ensure no updates as the target key `boo` doesn't exists.
		{
			name: "UpdateKeyFromExistingAttribute",
			inputAttributes: map[string]any{
				"foo": "there is a party over here",
				"boo": "not here",
			},
			expectedAttributes: map[string]any{
				"foo": "there is a party over here",
				"boo": "there is a party over here",
			},
		},
	}

	cfg := &Settings{
		Actions: []ActionKeyValue{
			{Key: "boo", Action: UPDATE, FromAttribute: "foo"},
		},
	}

	ap, err := NewAttrProc(cfg)
	require.Nil(t, err)
	require.NotNil(t, ap)

	for _, tt := range testCases {
		runIndividualTestCase(t, tt, ap)
	}
}

func TestAttributes_UpsertValue(t *testing.T) {
	testCases := []testCase{
		// Ensure `region` is set for spans with no attributes.
		{
			name:            "UpsertNoAttributes",
			inputAttributes: map[string]any{},
			expectedAttributes: map[string]any{
				"region": "planet-earth",
			},
		},
		// Ensure `region` is inserted for spans with some attributes(the key doesn't exist).
		{
			name: "UpsertAttributeNoExist",
			inputAttributes: map[string]any{
				"mission": "to mars",
			},
			expectedAttributes: map[string]any{
				"mission": "to mars",
				"region":  "planet-earth",
			},
		},
		// Ensure `region` is updated for spans with the attribute key `region`.
		{
			name: "UpsertAttributeExists",
			inputAttributes: map[string]any{
				"mission": "to mars",
				"region":  "solar system",
			},
			expectedAttributes: map[string]any{
				"mission": "to mars",
				"region":  "planet-earth",
			},
		},
	}

	cfg := &Settings{
		Actions: []ActionKeyValue{
			{Key: "region", Action: UPSERT, Value: "planet-earth"},
		},
	}

	ap, err := NewAttrProc(cfg)
	require.Nil(t, err)
	require.NotNil(t, ap)

	for _, tt := range testCases {
		runIndividualTestCase(t, tt, ap)
	}
}

func TestAttributes_Extract(t *testing.T) {
	testCases := []testCase{
		// Ensure `new_user_key` is not set for spans with no attributes.
		{
			name:               "UpsertEmptyAttributes",
			inputAttributes:    map[string]any{},
			expectedAttributes: map[string]any{},
		},
		// Ensure `new_user_key` is not inserted for spans with missing attribute `user_key`.
		{
			name: "No extract with no target key",
			inputAttributes: map[string]any{
				"boo": "ghosts are scary",
			},
			expectedAttributes: map[string]any{
				"boo": "ghosts are scary",
			},
		},
		// Ensure `new_user_key` is not inserted for spans with missing attribute `user_key`.
		{
			name: "No extract with non string target key",
			inputAttributes: map[string]any{
				"boo":      "ghosts are scary",
				"user_key": int64(1234),
			},
			expectedAttributes: map[string]any{
				"boo":      "ghosts are scary",
				"user_key": int64(1234),
			},
		},
		// Ensure `new_user_key` is not updated for spans with attribute
		// `user_key` because `user_key` does not match the regular expression.
		{
			name: "No extract with no pattern matching",
			inputAttributes: map[string]any{
				"user_key": "does not match",
				"boo":      "ghosts are scary",
			},
			expectedAttributes: map[string]any{
				"user_key": "does not match",
				"boo":      "ghosts are scary",
			},
		},
		// Ensure `new_user_key` is not updated for spans with attribute
		// `user_key` because `user_key` does not match all of the regular
		// expression.
		{
			name: "No extract with no pattern matching",
			inputAttributes: map[string]any{
				"user_key": "/api/v1/document/12345678/update",
				"boo":      "ghosts are scary",
			},
			expectedAttributes: map[string]any{
				"user_key": "/api/v1/document/12345678/update",
				"boo":      "ghosts are scary",
			},
		},
		// Ensure `new_user_key` and `version` is inserted for spans with attribute `user_key`.
		{
			name: "Extract insert new values.",
			inputAttributes: map[string]any{
				"user_key": "/api/v1/document/12345678/update/v1",
				"foo":      "casper the friendly ghost",
			},
			expectedAttributes: map[string]any{
				"user_key":     "/api/v1/document/12345678/update/v1",
				"new_user_key": "12345678",
				"version":      "v1",
				"foo":          "casper the friendly ghost",
			},
		},
		// Ensure `new_user_key` and `version` is updated for spans with attribute `user_key`.
		{
			name: "Extract updates existing values ",
			inputAttributes: map[string]any{
				"user_key":     "/api/v1/document/12345678/update/v1",
				"new_user_key": "2321",
				"version":      "na",
				"foo":          "casper the friendly ghost",
			},
			expectedAttributes: map[string]any{
				"user_key":     "/api/v1/document/12345678/update/v1",
				"new_user_key": "12345678",
				"version":      "v1",
				"foo":          "casper the friendly ghost",
			},
		},
		// Ensure `new_user_key` is updated and `version` is inserted for spans with attribute `user_key`.
		{
			name: "Extract upserts values",
			inputAttributes: map[string]any{
				"user_key":     "/api/v1/document/12345678/update/v1",
				"new_user_key": "2321",
				"foo":          "casper the friendly ghost",
			},
			expectedAttributes: map[string]any{
				"user_key":     "/api/v1/document/12345678/update/v1",
				"new_user_key": "12345678",
				"version":      "v1",
				"foo":          "casper the friendly ghost",
			},
		},
	}

	cfg := &Settings{
		Actions: []ActionKeyValue{

			{Key: "user_key", RegexPattern: "^\\/api\\/v1\\/document\\/(?P<new_user_key>.*)\\/update\\/(?P<version>.*)$", Action: EXTRACT},
		},
	}

	ap, err := NewAttrProc(cfg)
	require.Nil(t, err)
	require.NotNil(t, ap)

	for _, tt := range testCases {
		runIndividualTestCase(t, tt, ap)
	}
}

func TestAttributes_UpsertFromAttribute(t *testing.T) {

	testCases := []testCase{
		// Ensure `new_user_key` is not set for spans with no attributes.
		{
			name:               "UpsertEmptyAttributes",
			inputAttributes:    map[string]any{},
			expectedAttributes: map[string]any{},
		},
		// Ensure `new_user_key` is not inserted for spans with missing attribute `user_key`.
		{
			name: "UpsertFromAttributeNoExist",
			inputAttributes: map[string]any{
				"boo": "ghosts are scary",
			},
			expectedAttributes: map[string]any{
				"boo": "ghosts are scary",
			},
		},
		// Ensure `new_user_key` is inserted for spans with attribute `user_key`.
		{
			name: "UpsertFromAttributeExistsInsert",
			inputAttributes: map[string]any{
				"user_key": int64(2245),
				"foo":      "casper the friendly ghost",
			},
			expectedAttributes: map[string]any{
				"user_key":     int64(2245),
				"new_user_key": int64(2245),
				"foo":          "casper the friendly ghost",
			},
		},
		// Ensure `new_user_key` is updated for spans with attribute `user_key`.
		{
			name: "UpsertFromAttributeExistsUpdate",
			inputAttributes: map[string]any{
				"user_key":     int64(2245),
				"new_user_key": int64(5422),
				"foo":          "casper the friendly ghost",
			},
			expectedAttributes: map[string]any{
				"user_key":     int64(2245),
				"new_user_key": int64(2245),
				"foo":          "casper the friendly ghost",
			},
		},
	}

	cfg := &Settings{
		Actions: []ActionKeyValue{
			{Key: "new_user_key", Action: UPSERT, FromAttribute: "user_key"},
		},
	}

	ap, err := NewAttrProc(cfg)
	require.Nil(t, err)
	require.NotNil(t, ap)

	for _, tt := range testCases {
		runIndividualTestCase(t, tt, ap)
	}
}

func TestAttributes_Delete(t *testing.T) {
	testCases := []testCase{
		// Ensure the span contains no changes.
		{
			name:               "DeleteEmptyAttributes",
			inputAttributes:    map[string]any{},
			expectedAttributes: map[string]any{},
		},
		// Ensure the span contains no changes because the key doesn't exist.
		{
			name: "DeleteAttributeNoExist",
			inputAttributes: map[string]any{
				"boo": "ghosts are scary",
			},
			expectedAttributes: map[string]any{
				"boo": "ghosts are scary",
			},
		},
		// Ensure `duplicate_key` is deleted for spans with the attribute set.
		{
			name: "DeleteAttributeExists",
			inputAttributes: map[string]any{
				"duplicate_key": 3245.6,
				"original_key":  3245.6,
			},
			expectedAttributes: map[string]any{
				"original_key": 3245.6,
			},
		},
		// Ensure `duplicate_key` is deleted by regexp for spans with the attribute set.
		{
			name: "DeleteAttributeExists",
			inputAttributes: map[string]any{
				"duplicate_key_a":   3245.6,
				"duplicate_key_b":   3245.6,
				"duplicate_key_c":   3245.6,
				"original_key":      3245.6,
				"not_duplicate_key": 3246.6,
			},
			expectedAttributes: map[string]any{
				"original_key":      3245.6,
				"not_duplicate_key": 3246.6,
			},
		},
	}

	cfg := &Settings{
		Actions: []ActionKeyValue{
			{Key: "duplicate_key", RegexPattern: "^duplicate_key_.", Action: DELETE},
		},
	}

	ap, err := NewAttrProc(cfg)
	require.Nil(t, err)
	require.NotNil(t, ap)

	for _, tt := range testCases {
		runIndividualTestCase(t, tt, ap)
	}
}

func TestAttributes_Delete_Regexp(t *testing.T) {
	testCases := []testCase{
		// Ensure the span contains no changes.
		{
			name:               "DeleteEmptyAttributes",
			inputAttributes:    map[string]any{},
			expectedAttributes: map[string]any{},
		},
		// Ensure the span contains no changes because the key doesn't exist.
		{
			name: "DeleteAttributeNoExist",
			inputAttributes: map[string]any{
				"boo": "ghosts are scary",
			},
			expectedAttributes: map[string]any{
				"boo": "ghosts are scary",
			},
		},
		// Ensure `duplicate_key` is deleted for spans with the attribute set.
		{
			name: "DeleteAttributeExists",
			inputAttributes: map[string]any{
				"duplicate_key": 3245.6,
				"original_key":  3245.6,
			},
			expectedAttributes: map[string]any{
				"original_key": 3245.6,
			},
		},
	}

	cfg := &Settings{
		Actions: []ActionKeyValue{
			{RegexPattern: "duplicate.*", Action: DELETE},
		},
	}

	ap, err := NewAttrProc(cfg)
	require.Nil(t, err)
	require.NotNil(t, ap)

	for _, tt := range testCases {
		runIndividualTestCase(t, tt, ap)
	}
}

func TestAttributes_HashValue(t *testing.T) {
	intVal := int64(24)
	intBytes := make([]byte, int64ByteSize)
	binary.LittleEndian.PutUint64(intBytes, uint64(intVal))

	doubleVal := 2.4
	doubleBytes := make([]byte, float64ByteSize)
	binary.LittleEndian.PutUint64(doubleBytes, math.Float64bits(doubleVal))

	testCases := []testCase{
		// Ensure no changes to the span as there is no attributes map.
		{
			name:               "HashNoAttributes",
			inputAttributes:    map[string]any{},
			expectedAttributes: map[string]any{},
		},
		// Ensure no changes to the span as the key does not exist.
		{
			name: "HashKeyNoExist",
			inputAttributes: map[string]any{
				"boo": "foo",
			},
			expectedAttributes: map[string]any{
				"boo": "foo",
			},
		},
		// Ensure string data types are hashed correctly
		{
			name: "HashString",
			inputAttributes: map[string]any{
				"updateme": "foo",
			},
			expectedAttributes: map[string]any{
				"updateme": hash([]byte("foo")),
			},
		},
		// Ensure int data types are hashed correctly
		{
			name: "HashInt",
			inputAttributes: map[string]any{
				"updateme": intVal,
			},
			expectedAttributes: map[string]any{
				"updateme": hash(intBytes),
			},
		},
		// Ensure double data types are hashed correctly
		{
			name: "HashDouble",
			inputAttributes: map[string]any{
				"updateme": doubleVal,
			},
			expectedAttributes: map[string]any{
				"updateme": hash(doubleBytes),
			},
		},
		// Ensure bool data types are hashed correctly
		{
			name: "HashBoolTrue",
			inputAttributes: map[string]any{
				"updateme": true,
			},
			expectedAttributes: map[string]any{
				"updateme": hash([]byte{1}),
			},
		},
		// Ensure bool data types are hashed correctly
		{
			name: "HashBoolFalse",
			inputAttributes: map[string]any{
				"updateme": false,
			},
			expectedAttributes: map[string]any{
				"updateme": hash([]byte{0}),
			},
		},
		// Ensure regex pattern is being used
		{
			name: "HashRegex",
			inputAttributes: map[string]any{
				"updatemebyregexp":      false,
				"donotupdatemebyregexp": false,
			},
			expectedAttributes: map[string]any{
				"updatemebyregexp":      hash([]byte{0}),
				"donotupdatemebyregexp": false,
			},
		},
	}

	cfg := &Settings{
		Actions: []ActionKeyValue{
			{Key: "updateme", RegexPattern: "^updatemeby.*", Action: HASH},
		},
	}

	ap, err := NewAttrProc(cfg)
	require.Nil(t, err)
	require.NotNil(t, ap)

	for _, tt := range testCases {
		runIndividualTestCase(t, tt, ap)
	}
}

func TestAttributes_FromAttributeNoChange(t *testing.T) {
	tc := testCase{
		name: "FromAttributeNoChange",
		inputAttributes: map[string]any{
			"boo": "ghosts are scary",
		},
		expectedAttributes: map[string]any{
			"boo": "ghosts are scary",
		},
	}

	cfg := &Settings{
		Actions: []ActionKeyValue{
			{Key: "boo", Action: INSERT, FromAttribute: "boo"},
			{Key: "boo", Action: UPDATE, FromAttribute: "boo"},
			{Key: "boo", Action: UPSERT, FromAttribute: "boo"},
		},
	}

	ap, err := NewAttrProc(cfg)
	require.Nil(t, err)
	require.NotNil(t, ap)

	runIndividualTestCase(t, tc, ap)
}

func TestAttributes_Ordering(t *testing.T) {
	testCases := []testCase{
		// For this example, the operations performed are
		// 1. insert `operation`: `default`
		// 2. insert `svc.operation`: `default`
		// 3. delete `operation`.
		{
			name: "OrderingApplyAllSteps",
			inputAttributes: map[string]any{
				"foo": "casper the friendly ghost",
			},
			expectedAttributes: map[string]any{
				"foo":           "casper the friendly ghost",
				"svc.operation": "default",
			},
		},
		// For this example, the operations performed are
		// 1. do nothing for the first action of insert `operation`: `default`
		// 2. insert `svc.operation`: `arithmetic`
		// 3. delete `operation`.
		{
			name: "OrderingOperationExists",
			inputAttributes: map[string]any{
				"foo":       "casper the friendly ghost",
				"operation": "arithmetic",
			},
			expectedAttributes: map[string]any{
				"foo":           "casper the friendly ghost",
				"svc.operation": "arithmetic",
			},
		},

		// For this example, the operations performed are
		// 1. insert `operation`: `default`
		// 2. update `svc.operation` to `default`
		// 3. delete `operation`.
		{
			name: "OrderingSvcOperationExists",
			inputAttributes: map[string]any{
				"foo":           "casper the friendly ghost",
				"svc.operation": "some value",
			},
			expectedAttributes: map[string]any{
				"foo":           "casper the friendly ghost",
				"svc.operation": "default",
			},
		},

		// For this example, the operations performed are
		// 1. do nothing for the first action of insert `operation`: `default`
		// 2. update `svc.operation` to `arithmetic`
		// 3. delete `operation`.
		{
			name: "OrderingBothAttributesExist",
			inputAttributes: map[string]any{
				"foo":           "casper the friendly ghost",
				"operation":     "arithmetic",
				"svc.operation": "add",
			},
			expectedAttributes: map[string]any{
				"foo":           "casper the friendly ghost",
				"svc.operation": "arithmetic",
			},
		},
	}

	cfg := &Settings{
		Actions: []ActionKeyValue{
			{Key: "operation", Action: INSERT, Value: "default"},
			{Key: "svc.operation", Action: UPSERT, FromAttribute: "operation"},
			{Key: "operation", Action: DELETE},
		},
	}

	ap, err := NewAttrProc(cfg)
	require.Nil(t, err)
	require.NotNil(t, ap)

	for _, tt := range testCases {
		runIndividualTestCase(t, tt, ap)
	}
}

func TestInvalidConfig(t *testing.T) {
	testcase := []struct {
		name        string
		actionLists []ActionKeyValue
		errorString string
	}{
		{
			name: "missing key",
			actionLists: []ActionKeyValue{
				{Key: "one", Action: DELETE},
				{Key: "", Value: 123, Action: UPSERT},
			},
			errorString: "error creating AttrProc due to missing required field \"key\" at the 1-th actions",
		},
		{
			name: "invalid action",
			actionLists: []ActionKeyValue{
				{Key: "invalid", Action: "invalid"},
			},
			errorString: "error creating AttrProc due to unsupported action \"invalid\" at the 0-th actions",
		},
		{
			name: "unsupported value",
			actionLists: []ActionKeyValue{
				{Key: "UnsupportedValue", Value: []int{}, Action: UPSERT},
			},
			errorString: "<Invalid value type []int>",
		},
		{
			name: "missing value or from attribute",
			actionLists: []ActionKeyValue{
				{Key: "MissingValueFromAttributes", Action: INSERT},
			},
			errorString: "error creating AttrProc. Either field \"value\", \"from_attribute\" or \"from_context\" setting must be specified for 0-th action",
		},
		{
			name: "both set value and from attribute",
			actionLists: []ActionKeyValue{
				{Key: "BothSet", Value: 123, FromAttribute: "aa", Action: UPSERT},
			},
			errorString: "error creating AttrProc due to multiple value sources being set at the 0-th actions",
		},
		{
			name: "pattern shouldn't be specified",
			actionLists: []ActionKeyValue{
				{Key: "key", RegexPattern: "(?P<operation_website>.*?)$", FromAttribute: "aa", Action: INSERT},
			},
			errorString: "error creating AttrProc. Action \"insert\" does not use the \"pattern\" field. This must not be specified for 0-th action",
		},
		{
			name: "missing rule for extract",
			actionLists: []ActionKeyValue{
				{Key: "aa", Action: EXTRACT},
			},
			errorString: "error creating AttrProc due to missing required field \"pattern\" for action \"extract\" at the 0-th action",
		},
		{name: "set value for extract",
			actionLists: []ActionKeyValue{
				{Key: "Key", RegexPattern: "(?P<operation_website>.*?)$", Value: "value", Action: EXTRACT},
			},
			errorString: "error creating AttrProc. Action \"extract\" does not use a value source field. These must not be specified for 0-th action",
		},
		{
			name: "set from attribute for extract",
			actionLists: []ActionKeyValue{
				{Key: "key", RegexPattern: "(?P<operation_website>.*?)$", FromAttribute: "aa", Action: EXTRACT},
			},
			errorString: "error creating AttrProc. Action \"extract\" does not use a value source field. These must not be specified for 0-th action",
		},
		{
			name: "invalid regex",
			actionLists: []ActionKeyValue{
				{Key: "aa", RegexPattern: "(?P<invalid.regex>.*?)$", Action: EXTRACT},
			},
			errorString: "error creating AttrProc. Field \"pattern\" has invalid pattern: \"(?P<invalid.regex>.*?)$\" to be set at the 0-th actions",
		},
		{
			name: "regex with unnamed capture group",
			actionLists: []ActionKeyValue{
				{Key: "aa", RegexPattern: ".*$", Action: EXTRACT},
			},
			errorString: "error creating AttrProc. Field \"pattern\" contains no named matcher groups at the 0-th actions",
		},
		{
			name: "regex with one unnamed capture groups",
			actionLists: []ActionKeyValue{
				{Key: "aa", RegexPattern: "^\\/api\\/v1\\/document\\/(?P<new_user_key>.*)\\/update\\/(.*)$", Action: EXTRACT},
			},
			errorString: "error creating AttrProc. Field \"pattern\" contains at least one unnamed matcher group at the 0-th actions",
		},
	}

	for _, tc := range testcase {
		t.Run(tc.name, func(t *testing.T) {
			ap, err := NewAttrProc(&Settings{Actions: tc.actionLists})
			assert.Nil(t, ap)
			assert.EqualValues(t, errors.New(tc.errorString), err)
		})
	}
}

func TestValidConfiguration(t *testing.T) {
	cfg := &Settings{
		Actions: []ActionKeyValue{
			{Key: "one", Action: "Delete"},
			{Key: "two", Value: 123, Action: "INSERT"},
			{Key: "three", FromAttribute: "two", Action: "upDaTE"},
			{Key: "five", FromAttribute: "two", Action: "upsert"},
			{Key: "two", RegexPattern: "^\\/api\\/v1\\/document\\/(?P<documentId>.*)\\/update$", Action: "EXTRact"},
		},
	}
	ap, err := NewAttrProc(cfg)
	require.NoError(t, err)

	av := pcommon.NewValueInt(123)
	compiledRegex := regexp.MustCompile(`^\/api\/v1\/document\/(?P<documentId>.*)\/update$`)
	assert.Equal(t, []attributeAction{
		{Key: "one", Action: DELETE},
		{Key: "two", Action: INSERT,
			AttributeValue: &av,
		},
		{Key: "three", FromAttribute: "two", Action: UPDATE},
		{Key: "five", FromAttribute: "two", Action: UPSERT},
		{Key: "two", Regex: compiledRegex, AttrNames: []string{"", "documentId"}, Action: EXTRACT},
	}, ap.actions)

}

func hash(b []byte) string {
	if enableSha256Gate.IsEnabled() {
		return sha2Hash(b)
	}
	return sha1Hash(b)
}

func sha1Hash(b []byte) string {
	// #nosec
	h := sha1.New()
	h.Write(b)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func sha2Hash(b []byte) string {
	h := sha256.New()
	h.Write(b)
	return fmt.Sprintf("%x", h.Sum(nil))
}

type mockInfoAuth map[string]any

func (a mockInfoAuth) GetAttribute(name string) any {
	return a[name]
}

func (a mockInfoAuth) GetAttributeNames() []string {
	names := make([]string, 0, len(a))
	for name := range a {
		names = append(names, name)
	}
	return names
}

func TestFromContext(t *testing.T) {

	mdCtx := client.NewContext(context.TODO(), client.Info{
		Metadata: client.NewMetadata(map[string][]string{
			"source_single_val":   {"single_val"},
			"source_multiple_val": {"first_val", "second_val"},
		}),
		Auth: mockInfoAuth{
			"source_auth_val": "auth_val",
		},
	})

	testCases := []struct {
		name               string
		ctx                context.Context
		expectedAttributes map[string]any
		action             *ActionKeyValue
	}{
		{
			name:               "no_metadata",
			ctx:                context.TODO(),
			expectedAttributes: map[string]any{},
			action:             &ActionKeyValue{Key: "dest", FromContext: "source", Action: INSERT},
		},
		{
			name:               "no_value",
			ctx:                mdCtx,
			expectedAttributes: map[string]any{},
			action:             &ActionKeyValue{Key: "dest", FromContext: "source", Action: INSERT},
		},
		{
			name:               "single_value",
			ctx:                mdCtx,
			expectedAttributes: map[string]any{"dest": "single_val"},
			action:             &ActionKeyValue{Key: "dest", FromContext: "source_single_val", Action: INSERT},
		},
		{
			name:               "multiple_values",
			ctx:                mdCtx,
			expectedAttributes: map[string]any{"dest": "first_val;second_val"},
			action:             &ActionKeyValue{Key: "dest", FromContext: "source_multiple_val", Action: INSERT},
		},
		{
			name:               "with_metadata_prefix_single_value",
			ctx:                mdCtx,
			expectedAttributes: map[string]any{"dest": "single_val"},
			action:             &ActionKeyValue{Key: "dest", FromContext: "metadata.source_single_val", Action: INSERT},
		},
		{
			name:               "with_auth_prefix_single_value",
			ctx:                mdCtx,
			expectedAttributes: map[string]any{"dest": "auth_val"},
			action:             &ActionKeyValue{Key: "dest", FromContext: "auth.source_auth_val", Action: INSERT},
		},
		{
			name:               "with_auth_prefix_no_value",
			ctx:                mdCtx,
			expectedAttributes: map[string]any{},
			action:             &ActionKeyValue{Key: "dest", FromContext: "auth.unknown_val", Action: INSERT},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ap, err := NewAttrProc(&Settings{
				Actions: []ActionKeyValue{*tc.action},
			})
			require.Nil(t, err)
			require.NotNil(t, ap)
			attrMap := pcommon.NewMap()
			ap.Process(tc.ctx, nil, attrMap)
			require.Equal(t, tc.expectedAttributes, attrMap.AsRaw())
		})
	}
}
