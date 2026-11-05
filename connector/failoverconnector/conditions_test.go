// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorConditionShouldFailover(t *testing.T) {
	cond := &ErrorCondition{Contains: []string{"network failure", "connection refused"}}

	assert.False(t, cond.ShouldFailover(nil), "nil error must never trigger failover")
	assert.True(t, cond.ShouldFailover(errors.New("connection reset: network failure while exporting")),
		"error containing the substring must trigger failover")
	assert.False(t, cond.ShouldFailover(errors.New("sending_queue is full: data dropped")),
		"error without the substring must not trigger failover")
	assert.True(t, cond.ShouldFailover(errors.New("Network Failure")),
		"matching must be case-insensitive")
	assert.True(t, cond.ShouldFailover(errors.New("CONNECTION REFUSED")),
		"any configured substring may match")
}

func TestErrorConditionValidate(t *testing.T) {
	assert.NoError(t, (&ErrorCondition{Contains: []string{"network failure"}}).Validate())
	assert.ErrorIs(t, (&ErrorCondition{}).Validate(), errEmptyErrorContains)
	assert.ErrorIs(t, (&ErrorCondition{Contains: []string{""}}).Validate(), errEmptyErrorContains)
	assert.ErrorIs(t, (&ErrorCondition{Contains: []string{"network failure", ""}}).Validate(), errEmptyErrorContains)
}

func TestConditionsConfigValidate(t *testing.T) {
	assert.ErrorIs(t, (&ConditionsConfig{}).Validate(), errNoConditionDefined)
	assert.NoError(t, (&ConditionsConfig{ErrorCond: &ErrorCondition{}}).Validate())
	assert.NoError(t, (&ConditionsConfig{ErrorCond: &ErrorCondition{Contains: []string{"network failure"}}}).Validate())
}

func TestBuildCondition(t *testing.T) {
	assert.Nil(t, buildCondition(nil), "nil config must build no condition")

	cond := buildCondition(&ConditionsConfig{ErrorCond: &ErrorCondition{Contains: []string{"network failure"}}})
	require.NotNil(t, cond)
	assert.True(t, cond.ShouldFailover(errors.New("network failure")))
	assert.False(t, cond.ShouldFailover(errors.New("queue full")))
}
