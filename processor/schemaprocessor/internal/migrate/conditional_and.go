// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package migrate // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/migrate"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/otel/schema/v1.0/ast"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/alias"
)

type ResourceTestFunc[T alias.Resource] func(resource T) bool

type ConditionalLambdaAttributeSet[T alias.Resource] struct {
	testFunc ResourceTestFunc[T]
	attrs *AttributeChangeSet
}

type ConditionalLambdaAttributeSetSlice[T alias.Resource] []*ConditionalLambdaAttributeSet[T]

func NewConditionalLambdaAttributeSet[T alias.Resource](mappings ast.AttributeMap, testFunc ResourceTestFunc[T]) *ConditionalLambdaAttributeSet[T] {

	return &ConditionalLambdaAttributeSet[T]{
		testFunc:    testFunc,
		attrs: NewAttributeChangeSet(mappings),
	}
}

func (ca *ConditionalLambdaAttributeSet[T]) Apply(attrs pcommon.Map, resource T) (errs error) {
	if ca.check(resource) {
		errs = ca.attrs.Apply(attrs)
	}
	return errs
}

func (ca *ConditionalLambdaAttributeSet[T]) Rollback(attrs pcommon.Map, resource T) (errs error) {
	if ca.check(resource) {
		errs = ca.attrs.Rollback(attrs)
	}
	return errs
}

func (ca *ConditionalLambdaAttributeSet[T]) check(resource T) bool {
	if resource == nil {
		return true
	}
	return ca.testFunc(resource)
}

func NewConditionalLambdaAttributeSetSlice[T alias.Resource](conditions ...*ConditionalLambdaAttributeSet[T]) *ConditionalLambdaAttributeSetSlice[T] {
	values := new(ConditionalLambdaAttributeSetSlice[T])
	for _, c := range conditions {
		(*values) = append((*values), c)
	}
	return values
}

func (slice *ConditionalLambdaAttributeSetSlice[T]) Apply(attrs pcommon.Map, resource T) error {
	return slice.do(StateSelectorApply, attrs, resource)
}

func (slice *ConditionalLambdaAttributeSetSlice[T]) Rollback(attrs pcommon.Map, resource T) error {
	return slice.do(StateSelectorRollback, attrs, resource)
}

func (slice *ConditionalLambdaAttributeSetSlice[T]) do(ss StateSelector, attrs pcommon.Map, resource T) (errs error) {
	for i := 0; i < len((*slice)); i++ {
		switch ss {
		case StateSelectorApply:
			errs = multierr.Append(errs, (*slice)[i].Apply(attrs, resource))
		case StateSelectorRollback:
			errs = multierr.Append(errs, (*slice)[len((*slice))-i-1].Rollback(attrs, resource))
		}
	}
	return errs
}
