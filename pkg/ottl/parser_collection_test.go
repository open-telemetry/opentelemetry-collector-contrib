// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
)

type mockStatementsGetter struct {
	values []string
}

func (s mockStatementsGetter) GetStatements() []string {
	return s.values
}

type mockFailingContextInferrer struct {
	err error
}

func (r *mockFailingContextInferrer) Infer(_ []string) (string, error) {
	return "", r.err
}

type mockSetArguments[K any] struct {
	Target Setter[K]
	Value  Getter[K]
}

func Test_NewParserCollection(t *testing.T) {
	settings := componenttest.NewNopTelemetrySettings()
	pc, err := NewParserCollection[mockStatementsGetter, any](settings)
	require.NoError(t, err)

	assert.NotNil(t, pc)
	assert.NotNil(t, pc.contextParsers)
	assert.NotNil(t, pc.contextInferrer)
	assert.Equal(t, settings, pc.Settings)
}

func Test_NewParserCollection_OptionError(t *testing.T) {
	_, err := NewParserCollection[mockStatementsGetter, any](
		componenttest.NewNopTelemetrySettings(),
		func(_ *ParserCollection[mockStatementsGetter, any]) error {
			return errors.New("option error")
		},
	)

	require.Error(t, err, "option error")
}

func Test_WithContextParser(t *testing.T) {
	parser := mockParser(t)
	converter := NewNopParsedStatementConverter[any, mockStatementsGetter]()

	option := WithContextParser("testContext", parser, converter)

	pc, err := NewParserCollection[mockStatementsGetter, any](componenttest.NewNopTelemetrySettings(), option)
	require.NoError(t, err)

	pw, exists := pc.contextParsers["testContext"]
	assert.True(t, exists)
	assert.NotNil(t, pw)
	assert.Equal(t, reflect.ValueOf(parser), pw.ottlParser.Value)
	assert.Equal(t, reflect.ValueOf(converter), reflect.Value(pw.statementsConverter))
}

func Test_WithParserCollectionErrorMode(t *testing.T) {
	pc, err := NewParserCollection[mockStatementsGetter, any](
		componenttest.NewNopTelemetrySettings(),
		WithParserCollectionErrorMode[mockStatementsGetter, any](PropagateError),
	)

	require.NoError(t, err)
	require.NotNil(t, pc)
	require.Equal(t, PropagateError, pc.ErrorMode)
}

func Test_WithParserCollectionContextInferrer(t *testing.T) {
	inferrer := NewStaticContextInferrer("foo")

	pc, err := NewParserCollection[mockStatementsGetter, any](
		componenttest.NewNopTelemetrySettings(),
		WithParserCollectionContextInferrer[mockStatementsGetter, any](inferrer),
	)

	require.NoError(t, err)
	require.NotNil(t, pc)
	assert.Equal(t, inferrer, pc.contextInferrer)
}

func Test_NopParsedStatementConverter(t *testing.T) {
	type dummyContext struct{}

	noop := NewNopParsedStatementConverter[dummyContext, mockStatementsGetter]()
	parsedStatements := []*Statement[dummyContext]{{}}
	convertedStatements, err := noop(nil, nil, "", mockStatementsGetter{values: []string{}}, parsedStatements)

	require.NoError(t, err)
	require.NotNil(t, convertedStatements)
	assert.Equal(t, parsedStatements, convertedStatements)
}

func Test_NewParserCollection_DefaultInferrer(t *testing.T) {
	pc, err := NewParserCollection[mockStatementsGetter, any](componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	require.NotNil(t, pc)
	require.NotNil(t, pc.contextInferrer)
}

func Test_ParseStatements_Success(t *testing.T) {
	parser := mockParser(t)

	pc, err := NewParserCollection(
		component.TelemetrySettings{},
		WithContextParser("foo", parser, NewNopParsedStatementConverter[any, mockStatementsGetter]()),
		WithParserCollectionContextInferrer[mockStatementsGetter, any](NewStaticContextInferrer("foo")),
	)

	require.NoError(t, err)

	statements := mockStatementsGetter{values: []string{`set(foo.attributes["bar"], "foo")`, `set(foo.attributes["bar"], "bar")`}}
	result, err := pc.ParseStatements(statements)
	require.NoError(t, err)

	assert.IsType(t, []*Statement[any]{}, result)
	assert.Len(t, result.([]*Statement[any]), 2)
	assert.NotNil(t, result)
}

func Test_ParseStatements_MultipleParsers_Success(t *testing.T) {
	fooParser := mockParser(t)
	barParser := mockParser(t)

	pc, err := NewParserCollection(
		component.TelemetrySettings{},
		WithContextParser("foo", fooParser, newFailingConverter()),
		WithContextParser("bar", barParser, NewNopParsedStatementConverter[any, mockStatementsGetter]()),
		WithParserCollectionContextInferrer[mockStatementsGetter, any](NewStaticContextInferrer("bar")),
	)

	require.NoError(t, err)

	statements := mockStatementsGetter{values: []string{`set(bar.attributes["bar"], "foo")`, `set(bar.attributes["bar"], "bar")`}}
	result, err := pc.ParseStatements(statements)
	require.NoError(t, err)

	assert.IsType(t, []*Statement[any]{}, result)
	assert.Len(t, result.([]*Statement[any]), 2)
	assert.NotNil(t, result)
}

func Test_ParseStatements_EmptyContextInferenceError(t *testing.T) {
	pc, err := NewParserCollection[mockStatementsGetter, any](component.TelemetrySettings{},
		WithParserCollectionContextInferrer[mockStatementsGetter, any](NewStaticContextInferrer("")),
	)
	require.NoError(t, err)

	statements := mockStatementsGetter{values: []string{`set(bar.attributes["bar"], "foo")`}}
	_, err = pc.ParseStatements(statements)

	assert.ErrorContains(t, err, "unable to infer context from statements")
}

func Test_ParseStatements_ContextInferenceError(t *testing.T) {
	pc, err := NewParserCollection[mockStatementsGetter, any](component.TelemetrySettings{},
		WithParserCollectionContextInferrer[mockStatementsGetter, any](&mockFailingContextInferrer{err: errors.New("inference error")}),
	)
	require.NoError(t, err)

	statements := mockStatementsGetter{values: []string{`set(bar.attributes["bar"], "foo")`}}
	_, err = pc.ParseStatements(statements)

	assert.EqualError(t, err, "inference error")
}

func Test_ParseStatements_UnknownContextError(t *testing.T) {
	pc, err := NewParserCollection[mockStatementsGetter, any](component.TelemetrySettings{},
		WithParserCollectionContextInferrer[mockStatementsGetter, any](NewStaticContextInferrer("foo")),
	)
	require.NoError(t, err)

	statements := mockStatementsGetter{values: []string{`set(foo.attributes["bar"], "foo")`}}
	_, err = pc.ParseStatements(statements)

	assert.ErrorContains(t, err, `unknown context "foo"`)
}

func Test_ParseStatements_ParseStatementsError(t *testing.T) {
	parser := mockParser(t)
	parser.pathParser = func(_ Path[any]) (GetSetter[any], error) {
		return nil, errors.New("parse statements error")
	}

	pc, err := NewParserCollection(
		component.TelemetrySettings{},
		WithContextParser("foo", parser, NewNopParsedStatementConverter[any, mockStatementsGetter]()),
		WithParserCollectionContextInferrer[mockStatementsGetter, any](NewStaticContextInferrer("foo")),
	)

	require.NoError(t, err)

	statements := mockStatementsGetter{values: []string{`set(foo.attributes["bar"], "foo")`}}
	_, err = pc.ParseStatements(statements)
	assert.ErrorContains(t, err, "parse statements error")
}

func Test_ParseStatements_ConverterError(t *testing.T) {
	parser := mockParser(t)
	converter := func(_ *ParserCollection[mockStatementsGetter, any], _ *Parser[any], _ string, _ mockStatementsGetter, _ []*Statement[any]) (any, error) {
		return nil, errors.New("converter error")
	}

	pc, err := NewParserCollection(
		component.TelemetrySettings{},
		WithContextParser("dummy", parser, converter),
		WithParserCollectionContextInferrer[mockStatementsGetter, any](NewStaticContextInferrer("dummy")),
	)

	require.NoError(t, err)

	statements := mockStatementsGetter{values: []string{`set(dummy.attributes["bar"], "foo")`}}
	_, err = pc.ParseStatements(statements)

	assert.EqualError(t, err, "converter error")
}

func mockParser(t *testing.T) *Parser[any] {
	mockSetFactory := NewFactory("set", &mockSetArguments[any]{},
		func(_ FunctionContext, _ Arguments) (ExprFunc[any], error) {
			return func(_ context.Context, _ any) (any, error) {
				return nil, nil
			}, nil
		})

	parser, err := NewParser(
		CreateFactoryMap[any](mockSetFactory),
		testParsePath[any],
		componenttest.NewNopTelemetrySettings(),
		WithEnumParser[any](testParseEnum),
	)

	require.NoError(t, err)
	return &parser
}

func newFailingConverter() ParsedStatementConverter[any, mockStatementsGetter, any] {
	return func(_ *ParserCollection[mockStatementsGetter, any], _ *Parser[any], _ string, _ mockStatementsGetter, _ []*Statement[any]) (any, error) {
		return nil, errors.New("failing converter")
	}
}
