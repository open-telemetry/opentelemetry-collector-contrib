// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildPolicyBodyDefaults(t *testing.T) {
	m := &ismManager{cfg: ISMConfig{}}
	body, err := m.buildPolicyBody(otelV1SpanIndexAlias)
	require.NoError(t, err)

	require.Len(t, body.Policy.States, 1)
	require.Len(t, body.Policy.States[0].Actions, 1)
	rollover := body.Policy.States[0].Actions[0].Rollover
	require.NotNil(t, rollover)
	assert.Equal(t, defaultRolloverMinSize, rollover.MinSize)
	assert.Equal(t, defaultRolloverMinIndexAge, rollover.MinIndexAge)

	require.Len(t, body.Policy.Template, 1)
	assert.Equal(t, []string{otelV1SpanIndexAlias + "-*"}, body.Policy.Template[0].IndexPatterns)
}

func TestBuildPolicyBodyCustomRollover(t *testing.T) {
	m := &ismManager{cfg: ISMConfig{RolloverMinSize: "10gb", RolloverMinIndexAge: "1h"}}
	body, err := m.buildPolicyBody(otelV1LogsIndexAlias)
	require.NoError(t, err)

	rollover := body.Policy.States[0].Actions[0].Rollover
	require.NotNil(t, rollover)
	assert.Equal(t, "10gb", rollover.MinSize)
	assert.Equal(t, "1h", rollover.MinIndexAge)
}

func TestBuildPolicyBodyFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.json")
	content := `{"policy":{"description":"custom","default_state":"hot","states":[{"name":"hot","actions":[{"rollover":{"min_size":"5gb"}}]}]}}`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	m := &ismManager{cfg: ISMConfig{PolicyFile: path}}
	body, err := m.buildPolicyBody(otelV1SpanIndexAlias)
	require.NoError(t, err)

	assert.Equal(t, "custom", body.Policy.Description)
	assert.Equal(t, "hot", body.Policy.DefaultState)
	require.Len(t, body.Policy.States, 1)
	assert.Equal(t, "5gb", body.Policy.States[0].Actions[0].Rollover.MinSize)
}

func TestBuildPolicyBodyFromMissingFile(t *testing.T) {
	m := &ismManager{cfg: ISMConfig{PolicyFile: "/does/not/exist.json"}}
	_, err := m.buildPolicyBody(otelV1SpanIndexAlias)
	assert.ErrorContains(t, err, "reading ISM policy file")
}
