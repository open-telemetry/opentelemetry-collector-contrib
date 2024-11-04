// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"

import (
	"fmt"
	"reflect"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

var _ interface {
	ParseStatements(statements []string) ([]*Statement[any], error)
} = (*Parser[any])(nil)

var _ ParsedStatementConverter[any, StatementsGetter, any] = func(
	_ *ParserCollection[StatementsGetter, any],
	_ *Parser[any],
	_ string,
	_ StatementsGetter,
	_ []*Statement[any]) (any, error) {
	return nil, nil
}

// StatementsGetter represents a set of statements to be parsed
type StatementsGetter interface {
	// GetStatements retrieves the OTTL statements to be parsed
	GetStatements() []string
}

// ottlParserWrapper wraps an ottl.Parser using reflection, so it can invoke exported
// methods without knowing its generic type (transform context).
type ottlParserWrapper[S StatementsGetter] struct {
	parser                         reflect.Value
	prependContextToStatementPaths func(context string, statement string) (string, error)
}

func newParserWrapper[K any, S StatementsGetter](parser *Parser[K]) *ottlParserWrapper[S] {
	return &ottlParserWrapper[S]{
		parser:                         reflect.ValueOf(parser),
		prependContextToStatementPaths: parser.prependContextToStatementPaths,
	}
}

func (g *ottlParserWrapper[S]) parseStatements(statements []string) (reflect.Value, error) {
	method := g.parser.MethodByName("ParseStatements")
	parseStatementsRes := method.Call([]reflect.Value{reflect.ValueOf(statements)})
	err := parseStatementsRes[1]
	if !err.IsNil() {
		return reflect.Value{}, err.Interface().(error)
	}
	return parseStatementsRes[0], nil
}

func (g *ottlParserWrapper[S]) prependContextToStatementsPaths(context string, statements []string) ([]string, error) {
	result := make([]string, 0, len(statements))
	for _, s := range statements {
		prependedStatement, err := g.prependContextToStatementPaths(context, s)
		if err != nil {
			return nil, err
		}
		result = append(result, prependedStatement)
	}
	return result, nil
}

// statementsConverterWrapper is reflection-based wrapper to the ParsedStatementConverter function,
// which does not require knowing all generic parameters to be called.
type statementsConverterWrapper[S StatementsGetter] reflect.Value

func newStatementsConverterWrapper[K any, S StatementsGetter, R any](converter ParsedStatementConverter[K, S, R]) statementsConverterWrapper[S] {
	return statementsConverterWrapper[S](reflect.ValueOf(converter))
}

func (s statementsConverterWrapper[S]) call(
	parserCollection reflect.Value,
	ottlParser *ottlParserWrapper[S],
	context string,
	statements S,
	parsedStatements reflect.Value,
) (reflect.Value, error) {
	result := reflect.Value(s).Call([]reflect.Value{
		parserCollection,
		ottlParser.parser,
		reflect.ValueOf(context),
		reflect.ValueOf(statements),
		parsedStatements,
	})

	resultValue := result[0]
	resultError := result[1]
	if !resultError.IsNil() {
		return reflect.Value{}, resultError.Interface().(error)
	}

	return resultValue, nil
}

// parserCollectionParser holds an ottlParserWrapper and its respectively
// statementsConverter function.
type parserCollectionParser[S StatementsGetter] struct {
	ottlParser          *ottlParserWrapper[S]
	statementsConverter statementsConverterWrapper[S]
}

// ParserCollection is a configurable set of ottl.Parser that can handle multiple OTTL contexts
// parsings, inferring the context and choosing the right parser for the given statements.
type ParserCollection[S StatementsGetter, R any] struct {
	contextParsers           map[string]*parserCollectionParser[S]
	contextInferrer          contextInferrer
	modifiedStatementLogging bool
	Settings                 component.TelemetrySettings
	ErrorMode                ErrorMode
}

type ParserCollectionOption[S StatementsGetter, R any] func(*ParserCollection[S, R]) error

func NewParserCollection[S StatementsGetter, R any](
	settings component.TelemetrySettings,
	options ...ParserCollectionOption[S, R]) (*ParserCollection[S, R], error) {

	pc := &ParserCollection[S, R]{
		Settings:        settings,
		contextParsers:  map[string]*parserCollectionParser[S]{},
		contextInferrer: defaultPriorityContextInferrer(),
	}

	for _, op := range options {
		err := op(pc)
		if err != nil {
			return nil, err
		}
	}

	return pc, nil
}

// ParsedStatementConverter is a function that converts the parsed ottl.Statement[K] into
// a common representation to all parser collection contexts WithParserCollectionContext.
// Given each parser has its own transform context type, they must agree on a common type [R]
// so is can be returned by the ParserCollection.ParseStatements and ParserCollection.ParseStatementsWithContext
// functions.
type ParsedStatementConverter[T any, S StatementsGetter, R any] func(
	collection *ParserCollection[S, R],
	parser *Parser[T],
	context string,
	statements S,
	parsedStatements []*Statement[T],
) (R, error)

func newNopParsedStatementConverter[T any, S StatementsGetter]() ParsedStatementConverter[T, S, any] {
	return func(
		_ *ParserCollection[S, any],
		_ *Parser[T],
		_ string,
		_ S,
		parsedStatements []*Statement[T]) (any, error) {
		return parsedStatements, nil
	}
}

// WithParserCollectionContext configures an ottl.Parser for the given context.
// The provided ottl.Parser must be configured to support the provided context using
// the ottl.WithPathContextNames option.
func WithParserCollectionContext[K any, S StatementsGetter, R any](
	context string,
	parser *Parser[K],
	converter ParsedStatementConverter[K, S, R],
) ParserCollectionOption[S, R] {
	return func(mp *ParserCollection[S, R]) error {
		if _, ok := parser.pathContextNames[context]; !ok {
			return fmt.Errorf(`context "%s" must be a valid "%T" path context name`, context, parser)
		}
		mp.contextParsers[context] = &parserCollectionParser[S]{
			ottlParser:          newParserWrapper[K, S](parser),
			statementsConverter: newStatementsConverterWrapper(converter),
		}
		return nil
	}
}

// WithParserCollectionErrorMode has no effect on the ParserCollection, but might be used
// by the ParsedStatementConverter functions to handle/create StatementSequence.
func WithParserCollectionErrorMode[S StatementsGetter, R any](errorMode ErrorMode) ParserCollectionOption[S, R] {
	return func(tp *ParserCollection[S, R]) error {
		tp.ErrorMode = errorMode
		return nil
	}
}

// EnableParserCollectionModifiedStatementLogging controls the statements modification logs.
// When enabled, it logs any statements modifications performed by the parsing operations,
// instructing users to rewrite the statements accordingly.
func EnableParserCollectionModifiedStatementLogging[S StatementsGetter, R any](enabled bool) ParserCollectionOption[S, R] {
	return func(tp *ParserCollection[S, R]) error {
		tp.modifiedStatementLogging = enabled
		return nil
	}
}

// ParseStatements parses the given statements into [R] using the configured context's ottl.Parser
// and subsequently calling the ParsedStatementConverter function.
// The statement's context is automatically inferred from the [Path.Context] values, choosing the
// highest priority context found.
// If no contexts are present in the statements, or if the inferred value is not supported by
// the [ParserCollection], it returns an error.
// If parsing the statements fails, it returns the underline [ottl.Parser.ParseStatements] error.
func (pc *ParserCollection[S, R]) ParseStatements(statements S) (R, error) {
	statementsValues := statements.GetStatements()
	inferredContext, err := pc.contextInferrer.infer(statementsValues)
	if err != nil {
		return *new(R), err
	}

	if inferredContext == "" {
		return *new(R), fmt.Errorf("unable to infer context from statements [%v], path's first segment must be a valid context name", statementsValues)
	}

	return pc.ParseStatementsWithContext(inferredContext, statements, false)
}

// ParseStatementsWithContext parses the given statements into [R] using the configured
// context's ottl.Parser and subsequently calling the ParsedStatementConverter function.
// Differently from ParseStatements, it uses the provided context and does not infer it
// automatically. The context valuer must be supported by the [ParserCollection],
// otherwise an error is returned.
// If the statement's Path does not provide their Path.Context value, the prependPathsContext
// argument should be set to true, so it rewrites the statements prepending the missing paths
// contexts.
// If parsing the statements fails, it returns the underline [ottl.Parser.ParseStatements] error.
func (pc *ParserCollection[S, R]) ParseStatementsWithContext(context string, statements S, prependPathsContext bool) (R, error) {
	contextParser, ok := pc.contextParsers[context]
	if !ok {
		return *new(R), fmt.Errorf(`unknown context "%s" for stataments: %v`, context, statements.GetStatements())
	}

	var err error
	var parsingStatements []string
	if prependPathsContext {
		originalStatements := statements.GetStatements()
		parsingStatements, err = contextParser.ottlParser.prependContextToStatementsPaths(context, originalStatements)
		if err != nil {
			return *new(R), err
		}
		if pc.modifiedStatementLogging {
			pc.logModifiedStatements(originalStatements, parsingStatements)
		}
	} else {
		parsingStatements = statements.GetStatements()
	}

	parsedStatements, err := contextParser.ottlParser.parseStatements(parsingStatements)
	if err != nil {
		return *new(R), err
	}

	convertedStatements, err := contextParser.statementsConverter.call(
		reflect.ValueOf(pc),
		contextParser.ottlParser,
		context,
		statements,
		parsedStatements,
	)

	if err != nil {
		return *new(R), err
	}

	return convertedStatements.Interface().(R), nil
}

func (pc *ParserCollection[S, R]) logModifiedStatements(originalStatements, modifiedStatements []string) {
	var fields []zap.Field
	for i, original := range originalStatements {
		if modifiedStatements[i] != original {
			statementKey := fmt.Sprintf("[%v]", i)
			fields = append(fields, zap.Dict(
				statementKey,
				zap.String("original", original),
				zap.String("modified", modifiedStatements[i])),
			)
		}
	}
	if len(fields) > 0 {
		pc.Settings.Logger.Info("one or more statements were modified to include their paths context, please rewrite them accordingly", zap.Dict("statements", fields...))
	}
}
