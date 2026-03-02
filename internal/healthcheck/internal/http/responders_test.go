// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// var errUnserializable = errors.New("cannot marshal JSON")
var unserializableErrString = "cannot marshal unserializable"

type unserializable struct{}

func (*unserializable) MarshalJSON() ([]byte, error) {
	return nil, errors.New(unserializableErrString)
}

func TestRespondWithJSON(t *testing.T) {
	content := &unserializable{}
	w := httptest.NewRecorder()
	require.NoError(t, respondWithJSON(http.StatusOK, content, w))
	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), unserializableErrString)
}
