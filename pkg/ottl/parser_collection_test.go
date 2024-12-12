// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
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

func (r *mockFailingContextInferrer) infer(_ []string) (string, error) {
	return "", r.err
}

type mockStaticContextInferrer struct {
	value string
}

func (r *mockStaticContextInferrer) infer(_ []string) (string, error) {
	return r.value, nil
}

type mockSetArguments[K any] struct {
	Target Setter[K]
	Value  Getter[K]
}

func Test_NewParserCollection(t *testing.T) {
	settings := componenttest.NewNopTelemetrySettings()
	pc, err := NewParserCollection[any](settings)
	require.NoError(t, err)

	assert.NotNil(t, pc)
	assert.NotNil(t, pc.contextParsers)
	assert.NotNil(t, pc.contextInferrer)
}

func Test_NewParserCollection_OptionError(t *testing.T) {
	_, err := NewParserCollection[any](
		componenttest.NewNopTelemetrySettings(),
		func(_ *ParserCollection[any]) error {
			return errors.New("option error")
		},
	)

	require.Error(t, err, "option error")
}

func Test_WithParserCollectionContext(t *testing.T) {
	ps := mockParser(t, WithPathContextNames[any]([]string{"testContext"}))
	conv := newNopParsedStatementConverter[any]()
	option := WithParserCollectionContext("testContext", ps, conv)

	pc, err := NewParserCollection[any](componenttest.NewNopTelemetrySettings(), option)
	require.NoError(t, err)

	pw, exists := pc.contextParsers["testContext"]
	assert.True(t, exists)
	assert.NotNil(t, pw)
	assert.Equal(t, reflect.ValueOf(ps), pw.ottlParser.parser)
	assert.Equal(t, reflect.ValueOf(conv), reflect.Value(pw.statementsConverter))
}

func Test_WithParserCollectionContext_UnsupportedContext(t *testing.T) {
	ps := mockParser(t, WithPathContextNames[any]([]string{"foo"}))
	conv := newNopParsedStatementConverter[any]()
	option := WithParserCollectionContext("bar", ps, conv)

	_, err := NewParserCollection[any](componenttest.NewNopTelemetrySettings(), option)

	require.ErrorContains(t, err, `context "bar" must be a valid "*ottl.Parser[interface {}]" path context name`)
}

func Test_WithParserCollectionErrorMode(t *testing.T) {
	pc, err := NewParserCollection[any](
		componenttest.NewNopTelemetrySettings(),
		WithParserCollectionErrorMode[any](PropagateError),
	)

	require.NoError(t, err)
	require.NotNil(t, pc)
	require.Equal(t, PropagateError, pc.ErrorMode)
}

func Test_EnableParserCollectionModifiedStatementLogging_True(t *testing.T) {
	ps := mockParser(t, WithPathContextNames[any]([]string{"dummy"}))
	core, observedLogs := observer.New(zap.InfoLevel)
	telemetrySettings := componenttest.NewNopTelemetrySettings()
	telemetrySettings.Logger = zap.New(core)

	pc, err := NewParserCollection(
		telemetrySettings,
		WithParserCollectionContext("dummy", ps, newNopParsedStatementConverter[any]()),
		EnableParserCollectionModifiedStatementLogging[any](true),
	)
	require.NoError(t, err)

	originalStatements := []string{
		`set(attributes["foo"], "foo")`,
		`set(attributes["bar"], "bar")`,
	}

	_, err = pc.ParseStatementsWithContext("dummy", mockStatementsGetter{originalStatements}, true)
	require.NoError(t, err)

	logEntries := observedLogs.TakeAll()
	require.Len(t, logEntries, 1)
	logEntry := logEntries[0]
	require.Equal(t, zap.InfoLevel, logEntry.Level)
	require.Contains(t, logEntry.Message, "one or more statements were modified")
	logEntryStatements := logEntry.ContextMap()["statements"]
	require.NotNil(t, logEntryStatements)

	for i, originalStatement := range originalStatements {
		k := fmt.Sprintf("[%d]", i)
		logEntryStatementContext := logEntryStatements.(map[string]any)[k]
		require.Equal(t, logEntryStatementContext.(map[string]any)["original"], originalStatement)
		modifiedStatement, err := ps.prependContextToStatementPaths("dummy", originalStatement)
		require.NoError(t, err)
		require.Equal(t, logEntryStatementContext.(map[string]any)["modified"], modifiedStatement)
	}
}

func Test_EnableParserCollectionModifiedStatementLogging_False(t *testing.T) {
	ps := mockParser(t, WithPathContextNames[any]([]string{"dummy"}))
	core, observedLogs := observer.New(zap.InfoLevel)
	telemetrySettings := componenttest.NewNopTelemetrySettings()
	telemetrySettings.Logger = zap.New(core)

	pc, err := NewParserCollection(
		telemetrySettings,
		WithParserCollectionContext("dummy", ps, newNopParsedStatementConverter[any]()),
		EnableParserCollectionModifiedStatementLogging[any](false),
	)
	require.NoError(t, err)

	_, err = pc.ParseStatementsWithContext("dummy", mockStatementsGetter{[]string{`set(attributes["foo"], "foo")`}}, true)
	require.NoError(t, err)
	require.Empty(t, observedLogs.TakeAll())
}

func Test_NopParsedStatementConverter(t *testing.T) {
	type dummyContext struct{}

	noop := newNopParsedStatementConverter[dummyContext]()
	parsedStatements := []*Statement[dummyContext]{{}}
	convertedStatements, err := noop(nil, nil, "", mockStatementsGetter{values: []string{}}, parsedStatements)

	require.NoError(t, err)
	require.NotNil(t, convertedStatements)
	assert.Equal(t, parsedStatements, convertedStatements)
}

func Test_NewParserCollection_DefaultContextInferrer(t *testing.T) {
	pc, err := NewParserCollection[any](componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	require.NotNil(t, pc)
	require.NotNil(t, pc.contextInferrer)
}

func Test_ParseStatements_Success(t *testing.T) {
	ps := mockParser(t, WithPathContextNames[any]([]string{"foo"}))

	pc, err := NewParserCollection(
		component.TelemetrySettings{},
		WithParserCollectionContext("foo", ps, newNopParsedStatementConverter[any]()),
	)
	require.NoError(t, err)
	pc.contextInferrer = &mockStaticContextInferrer{"foo"}

	statements := mockStatementsGetter{values: []string{`set(foo.attributes["bar"], "foo")`, `set(foo.attributes["bar"], "bar")`}}
	result, err := pc.ParseStatements(statements)
	require.NoError(t, err)

	assert.IsType(t, []*Statement[any]{}, result)
	assert.Len(t, result.([]*Statement[any]), 2)
	assert.NotNil(t, result)
}

func Test_ParseStatements_MultipleContexts_Success(t *testing.T) {
	fooParser := mockParser(t, WithPathContextNames[any]([]string{"foo"}))
	barParser := mockParser(t, WithPathContextNames[any]([]string{"bar"}))
	failingConverter := func(
		_ *ParserCollection[any],
		_ *Parser[any],
		_ string,
		_ StatementsGetter,
		_ []*Statement[any],
	) (any, error) {
		return nil, errors.New("failing converter")
	}

	pc, err := NewParserCollection(
		component.TelemetrySettings{},
		WithParserCollectionContext("foo", fooParser, failingConverter),
		WithParserCollectionContext("bar", barParser, newNopParsedStatementConverter[any]()),
	)
	require.NoError(t, err)
	pc.contextInferrer = &mockStaticContextInferrer{"bar"}

	// The `foo` context is never used, so these statements will successfully parse.
	statements := mockStatementsGetter{values: []string{`set(bar.attributes["bar"], "foo")`, `set(bar.attributes["bar"], "bar")`}}
	result, err := pc.ParseStatements(statements)
	require.NoError(t, err)

	assert.IsType(t, []*Statement[any]{}, result)
	assert.Len(t, result.([]*Statement[any]), 2)
	assert.NotNil(t, result)
}

func Test_ParseStatements_NoContextInferredError(t *testing.T) {
	pc, err := NewParserCollection[any](component.TelemetrySettings{})
	require.NoError(t, err)
	pc.contextInferrer = &mockStaticContextInferrer{""}

	statements := mockStatementsGetter{values: []string{`set(bar.attributes["bar"], "foo")`}}
	_, err = pc.ParseStatements(statements)

	assert.ErrorContains(t, err, "unable to infer context from statements")
}

func Test_ParseStatements_ContextInferenceError(t *testing.T) {
	pc, err := NewParserCollection[any](component.TelemetrySettings{})
	require.NoError(t, err)
	pc.contextInferrer = &mockFailingContextInferrer{err: errors.New("inference error")}

	statements := mockStatementsGetter{values: []string{`set(bar.attributes["bar"], "foo")`}}
	_, err = pc.ParseStatements(statements)

	assert.EqualError(t, err, "inference error")
}

func Test_ParseStatements_UnknownContextError(t *testing.T) {
	pc, err := NewParserCollection[any](component.TelemetrySettings{})
	require.NoError(t, err)
	pc.contextInferrer = &mockStaticContextInferrer{"foo"}

	statements := mockStatementsGetter{values: []string{`set(foo.attributes["bar"], "foo")`}}
	_, err = pc.ParseStatements(statements)

	assert.ErrorContains(t, err, `unknown context "foo"`)
}

func Test_ParseStatements_ParseStatementsError(t *testing.T) {
	ps := mockParser(t, WithPathContextNames[any]([]string{"foo"}))
	ps.pathParser = func(_ Path[any]) (GetSetter[any], error) {
		return nil, errors.New("parse statements error")
	}

	pc, err := NewParserCollection(
		component.TelemetrySettings{},
		WithParserCollectionContext("foo", ps, newNopParsedStatementConverter[any]()),
	)
	require.NoError(t, err)
	pc.contextInferrer = &mockStaticContextInferrer{"foo"}

	statements := mockStatementsGetter{values: []string{`set(foo.attributes["bar"], "foo")`}}
	_, err = pc.ParseStatements(statements)
	assert.ErrorContains(t, err, "parse statements error")
}

func Test_ParseStatements_ConverterError(t *testing.T) {
	ps := mockParser(t, WithPathContextNames[any]([]string{"dummy"}))
	conv := func(_ *ParserCollection[any], _ *Parser[any], _ string, _ StatementsGetter, _ []*Statement[any]) (any, error) {
		return nil, errors.New("converter error")
	}

	pc, err := NewParserCollection(
		component.TelemetrySettings{},
		WithParserCollectionContext("dummy", ps, conv),
	)
	require.NoError(t, err)
	pc.contextInferrer = &mockStaticContextInferrer{"dummy"}

	statements := mockStatementsGetter{values: []string{`set(dummy.attributes["bar"], "foo")`}}
	_, err = pc.ParseStatements(statements)

	assert.EqualError(t, err, "converter error")
}

func Test_ParseStatements_ConverterNilReturn(t *testing.T) {
	ps := mockParser(t, WithPathContextNames[any]([]string{"dummy"}))
	conv := func(_ *ParserCollection[any], _ *Parser[any], _ string, _ StatementsGetter, _ []*Statement[any]) (any, error) {
		return nil, nil
	}

	pc, err := NewParserCollection(
		component.TelemetrySettings{},
		WithParserCollectionContext("dummy", ps, conv),
	)
	require.NoError(t, err)
	pc.contextInferrer = &mockStaticContextInferrer{"dummy"}

	statements := mockStatementsGetter{values: []string{`set(dummy.attributes["bar"], "foo")`}}
	result, err := pc.ParseStatements(statements)
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func Test_ParseStatements_StatementsConverterGetterType(t *testing.T) {
	ps := mockParser(t, WithPathContextNames[any]([]string{"dummy"}))
	statements := mockStatementsGetter{values: []string{`set(dummy.attributes["bar"], "foo")`}}
	conv := func(_ *ParserCollection[any], _ *Parser[any], _ string, statementsGetter StatementsGetter, _ []*Statement[any]) (any, error) {
		switch statementsGetter.(type) {
		case mockStatementsGetter:
			return statements, nil
		default:
			return nil, fmt.Errorf("invalid StatementsGetter type, expected: mockStatementsGetter, got: %T", statementsGetter)
		}
	}

	pc, err := NewParserCollection(component.TelemetrySettings{}, WithParserCollectionContext("dummy", ps, conv))
	require.NoError(t, err)
	pc.contextInferrer = &mockStaticContextInferrer{"dummy"}

	_, err = pc.ParseStatements(statements)
	require.NoError(t, err)
}

func Test_ParseStatementsWithContext_UnknownContextError(t *testing.T) {
	pc, err := NewParserCollection[any](component.TelemetrySettings{})
	require.NoError(t, err)

	statements := mockStatementsGetter{[]string{`set(attributes["bar"], "bar")`}}
	_, err = pc.ParseStatementsWithContext("bar", statements, false)

	assert.ErrorContains(t, err, `unknown context "bar"`)
}

func Test_ParseStatementsWithContext_PrependPathContext(t *testing.T) {
	ps := mockParser(t, WithPathContextNames[any]([]string{"dummy"}))
	pc, err := NewParserCollection(
		component.TelemetrySettings{},
		WithParserCollectionContext("dummy", ps, newNopParsedStatementConverter[any]()),
	)
	require.NoError(t, err)

	result, err := pc.ParseStatementsWithContext(
		"dummy",
		mockStatementsGetter{[]string{
			`set(attributes["foo"], "foo")`,
			`set(attributes["bar"], "bar")`,
		}},
		true,
	)

	require.NoError(t, err)
	require.Len(t, result, 2)
	parsedStatements := result.([]*Statement[any])
	assert.Equal(t, `set(dummy.attributes["foo"], "foo")`, parsedStatements[0].origText)
	assert.Equal(t, `set(dummy.attributes["bar"], "bar")`, parsedStatements[1].origText)
}

func Test_NewStatementsGetter(t *testing.T) {
	statements := []string{`set(foo, "bar")`, `set(bar, "foo")`}
	statementsGetter := NewStatementsGetter(statements)
	assert.Implements(t, (*StatementsGetter)(nil), statementsGetter)
	assert.Equal(t, statements, statementsGetter.GetStatements())
}

func mockParser(t *testing.T, options ...Option[any]) *Parser[any] {
	mockSetFactory := NewFactory("set", &mockSetArguments[any]{},
		func(_ FunctionContext, _ Arguments) (ExprFunc[any], error) {
			return func(_ context.Context, _ any) (any, error) {
				return nil, nil
			}, nil
		})

	ps, err := NewParser(
		CreateFactoryMap[any](mockSetFactory),
		testParsePath[any],
		componenttest.NewNopTelemetrySettings(),
		append([]Option[any]{
			WithEnumParser[any](testParseEnum),
		}, options...)...,
	)

	require.NoError(t, err)
	return &ps
}
