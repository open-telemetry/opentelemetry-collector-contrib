// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl

import (
	"fmt"
	"reflect"

	"go.opentelemetry.io/collector/component"
)

var _ ottlParser[any] = (*Parser[any])(nil)

var _ ParsedStatementConverter[any, StatementsGetter, any] = func(
	_ *ParserCollection[StatementsGetter, any],
	_ *Parser[any],
	_ string,
	_ StatementsGetter,
	_ []*Statement[any]) (any, error) {
	return nil, nil
}

type ottlParser[K any] interface {
	// ParseStatements is the same as Parser.ParseStatements
	ParseStatements(statements []string) ([]*Statement[K], error)
	// AppendStatementPathsContext is the same as Parser.AppendStatementPathsContext
	AppendStatementPathsContext(context string, statement string) (string, error)
}

// StatementsGetter represents the input statements to be parsed
type StatementsGetter interface {
	GetStatements() []string
}

type ottlParserWrapper[S StatementsGetter] struct {
	reflect.Value
}

func (g *ottlParserWrapper[S]) ParseStatements(statements []string) (reflect.Value, error) {
	method := g.MethodByName("ParseStatements")
	psr := method.Call([]reflect.Value{reflect.ValueOf(statements)})
	err := psr[1]
	if !err.IsNil() {
		return reflect.Value{}, err.Interface().(error)
	}
	return psr[0], nil
}

func (g *ottlParserWrapper[S]) AppendStatementPathsContext(context string, statement string) (string, error) {
	method := g.MethodByName("AppendStatementPathsContext")
	psr := method.Call([]reflect.Value{reflect.ValueOf(context), reflect.ValueOf(statement)})
	err := psr[1]
	if !err.IsNil() {
		return "", err.Interface().(error)
	}
	return psr[0].Interface().(string), nil
}

func newParserWrapper[K any, S StatementsGetter](parser *Parser[K]) *ottlParserWrapper[S] {
	return &ottlParserWrapper[S]{reflect.ValueOf(parser)}
}

type statementsConverterWrapper[S StatementsGetter] reflect.Value

func (s statementsConverterWrapper[S]) Call(in []reflect.Value) []reflect.Value {
	return reflect.Value(s).Call(in)
}

func newStatementsConverterWrapper[K any, S StatementsGetter, R any](converter ParsedStatementConverter[K, S, R]) statementsConverterWrapper[S] {
	return statementsConverterWrapper[S](reflect.ValueOf(converter))
}

type contextParserWrapper[S StatementsGetter] struct {
	ottlParser          *ottlParserWrapper[S]
	statementsConverter statementsConverterWrapper[S]
}

type ParserCollection[S StatementsGetter, R any] struct {
	contextParsers  map[string]*contextParserWrapper[S]
	contextInferrer ContextInferrer
	Settings        component.TelemetrySettings
	ErrorMode       ErrorMode
}

type ParserCollectionOption[S StatementsGetter, R any] func(*ParserCollection[S, R]) error

func NewParserCollection[S StatementsGetter, R any](
	settings component.TelemetrySettings,
	options ...ParserCollectionOption[S, R]) (*ParserCollection[S, R], error) {

	pc := &ParserCollection[S, R]{
		Settings:       settings,
		contextParsers: map[string]*contextParserWrapper[S]{},
	}

	for _, op := range options {
		err := op(pc)
		if err != nil {
			return nil, err
		}
	}

	if pc.contextInferrer == nil {
		pc.contextInferrer = NewDefaultContextInferrer()
	}

	return pc, nil
}

type ParsedStatementConverter[T any, S StatementsGetter, R any] func(
	collection *ParserCollection[S, R],
	parser *Parser[T],
	context string,
	statements S,
	parsedStatements []*Statement[T],
) (R, error)

func NewNopParsedStatementConverter[T any, S StatementsGetter]() ParsedStatementConverter[T, S, any] {
	return func(
		_ *ParserCollection[S, any],
		_ *Parser[T],
		_ string,
		_ S,
		parsedStatements []*Statement[T]) (any, error) {
		return parsedStatements, nil
	}
}

func WithContextParser[K any, S StatementsGetter, R any](
	context string,
	parser *Parser[K],
	converter ParsedStatementConverter[K, S, R],
) ParserCollectionOption[S, R] {
	return func(mp *ParserCollection[S, R]) error {
		if len(parser.pathContextNames) == 0 {
			WithPathContextNames[K]([]string{context})(parser)
		}
		mp.contextParsers[context] = &contextParserWrapper[S]{
			ottlParser:          newParserWrapper[K, S](parser),
			statementsConverter: newStatementsConverterWrapper(converter),
		}
		return nil
	}
}

func WithParserCollectionErrorMode[S StatementsGetter, R any](errorMode ErrorMode) ParserCollectionOption[S, R] {
	return func(tp *ParserCollection[S, R]) error {
		tp.ErrorMode = errorMode
		return nil
	}
}

func WithParserCollectionContextInferrer[S StatementsGetter, R any](contextInferrer ContextInferrer) ParserCollectionOption[S, R] {
	return func(tp *ParserCollection[S, R]) error {
		tp.contextInferrer = contextInferrer
		return nil
	}
}

func (pc *ParserCollection[S, R]) ParseStatements(statements S) (R, error) {
	zero := *new(R)
	statementsValues := statements.GetStatements()

	inferredContext, err := pc.contextInferrer.Infer(statementsValues)
	if err != nil {
		return zero, err
	}

	if inferredContext == "" {
		return zero, fmt.Errorf("unable to infer context from statements [%v], path's first segment must be a valid context name", statementsValues)
	}

	return pc.ParseStatementsWithContext(inferredContext, statements, false)
}

func (pc *ParserCollection[S, R]) ParseStatementsWithContext(context string, statements S, appendPathsContext bool) (R, error) {
	zero := *new(R)

	contextParser, ok := pc.contextParsers[context]
	if !ok {
		return zero, fmt.Errorf(`unknown context "%s" for stataments: %v`, context, statements.GetStatements())
	}

	var statementsValues []string
	if appendPathsContext {
		for _, s := range statements.GetStatements() {
			ctxStatement, err := contextParser.ottlParser.AppendStatementPathsContext(context, s)
			if err != nil {
				return zero, err
			}
			statementsValues = append(statementsValues, ctxStatement)
		}
	} else {
		statementsValues = statements.GetStatements()
	}

	parsedStatements, err := contextParser.ottlParser.ParseStatements(statementsValues)
	if err != nil {
		return zero, err
	}

	scr := contextParser.statementsConverter.Call([]reflect.Value{
		reflect.ValueOf(pc),
		contextParser.ottlParser.Value,
		reflect.ValueOf(context),
		reflect.ValueOf(statements),
		parsedStatements,
	})

	result := scr[0]
	converterErr := scr[1]
	if !converterErr.IsNil() {
		return zero, converterErr.Interface().(error)
	}

	return result.Interface().(R), nil
}
