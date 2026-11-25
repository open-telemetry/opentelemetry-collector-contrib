// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumererror"
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

func TestErrorConditionShouldFailoverPermanent(t *testing.T) {
	permanentErr := consumererror.NewPermanent(errors.New("network failure"))
	retryableErr := errors.New("network failure")

	onlyRetryable := &ErrorCondition{Permanent: new(bool)}
	assert.False(t, onlyRetryable.ShouldFailover(nil))
	assert.False(t, onlyRetryable.ShouldFailover(permanentErr))
	assert.True(t, onlyRetryable.ShouldFailover(retryableErr))

	permanent := true
	onlyPermanent := &ErrorCondition{Permanent: &permanent}
	assert.True(t, onlyPermanent.ShouldFailover(permanentErr))
	assert.False(t, onlyPermanent.ShouldFailover(retryableErr))
}

func TestErrorConditionShouldFailoverAndsChecks(t *testing.T) {
	cond := &ErrorCondition{Contains: []string{"network failure"}, Permanent: new(bool)}

	assert.True(t, cond.ShouldFailover(errors.New("network failure")),
		"error passing all checks must trigger failover")
	assert.False(t, cond.ShouldFailover(consumererror.NewPermanent(errors.New("network failure"))),
		"error failing the permanent check must not trigger failover")
	assert.False(t, cond.ShouldFailover(errors.New("queue full")),
		"error failing the contains check must not trigger failover")
}

func TestErrorConditionValidate(t *testing.T) {
	assert.NoError(t, (&ErrorCondition{Contains: []string{"network failure"}}).Validate())
	assert.NoError(t, (&ErrorCondition{Permanent: new(bool)}).Validate())
	assert.NoError(t, (&ErrorCondition{Contains: []string{"network failure"}, Permanent: new(bool)}).Validate())
	assert.ErrorIs(t, (&ErrorCondition{}).Validate(), errEmptyErrorCondition)
	assert.ErrorIs(t, (&ErrorCondition{Contains: []string{""}}).Validate(), errEmptyErrorContains)
	assert.ErrorIs(t, (&ErrorCondition{Contains: []string{"network failure", ""}}).Validate(), errEmptyErrorContains)
	assert.ErrorIs(t, (&ErrorCondition{Contains: []string{""}, Permanent: new(bool)}).Validate(), errEmptyErrorContains)
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
