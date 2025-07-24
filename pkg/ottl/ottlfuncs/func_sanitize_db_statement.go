// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"
	"strings"
	"unicode"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type SanitizeDBStatementArguments[K any] struct {
	Target ottl.StringGetter[K]
}

func NewSanitizeDBStatementFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("SanitizeDBStatement", &SanitizeDBStatementArguments[K]{}, createSanitizeDBStatementFunction[K])
}

func createSanitizeDBStatementFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*SanitizeDBStatementArguments[K])
	if !ok {
		return nil, errors.New("SanitizeDBStatementFactory args must be of type *SanitizeDBStatementArguments[K]")
	}

	return sanitizeDBStatement(args.Target), nil
}

type tokenType int

const (
	tokenUnknown tokenType = iota
	tokenWhitespace
	tokenIdentifier
	tokenNumber
	tokenString
	tokenOperator
	tokenPunctuation
	tokenComment
	tokenKeyword
)

type token struct {
	typ   tokenType
	value string
}

type sqlLexer struct {
	input string
	pos   int
	start int
}

func (l *sqlLexer) next() rune {
	if l.pos >= len(l.input) {
		return 0
	}
	r := rune(l.input[l.pos])
	l.pos++
	return r
}

func (l *sqlLexer) peek() rune {
	if l.pos >= len(l.input) {
		return 0
	}
	return rune(l.input[l.pos])
}

func (l *sqlLexer) backup() {
	if l.pos > 0 {
		l.pos--
	}
}

func (l *sqlLexer) emit(typ tokenType) token {
	tok := token{typ: typ, value: l.input[l.start:l.pos]}
	l.start = l.pos
	return tok
}

func (l *sqlLexer) skipWhitespace() token {
	for l.pos < len(l.input) {
		r := l.next()
		if !unicode.IsSpace(r) {
			l.backup()
			break
		}
	}
	return l.emit(tokenWhitespace)
}

func isHexDigit(r rune) bool {
	return unicode.IsDigit(r) || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

func (l *sqlLexer) scanNumber() token {
	// Handle 0x prefix for hex numbers
	if l.peek() == 'x' && l.input[l.start] == '0' {
		l.next() // consume 'x'
		hasHexDigit := false
		for {
			r := l.next()
			if r == 0 {
				break
			}
			if !isHexDigit(r) {
				l.backup()
				break
			}
			hasHexDigit = true
		}
		if hasHexDigit {
			return l.emit(tokenNumber)
		}
		// Not a valid hex number, reset position
		l.pos = l.start
	}

	// Handle regular numbers including scientific notation
	for {
		r := l.next()
		if r == 0 {
			break
		}
		if r == '.' || r == 'e' || r == 'E' || r == '-' || r == '+' || unicode.IsDigit(r) {
			continue
		}
		l.backup()
		break
	}
	return l.emit(tokenNumber)
}

func (l *sqlLexer) scanString() token {
	quote := rune(l.input[l.start])
	if quote == '$' {
		// Handle dollar quoted strings
		l.next() // consume first $
		l.next() // consume second $
		for {
			r := l.next()
			if r == 0 {
				break
			}
			if r == '$' && l.peek() == '$' {
				l.next() // consume second $
				break
			}
		}
		return l.emit(tokenString)
	}

	escaped := false
	for {
		r := l.next()
		if r == 0 {
			break
		}
		if r == quote && !escaped {
			if l.peek() == quote {
				l.next() // consume repeated quote
				escaped = false
				continue
			}
			break
		}
		if r == '\\' {
			escaped = !escaped
		} else {
			escaped = false
		}
	}
	return l.emit(tokenString)
}

func (l *sqlLexer) scanIdentifier() token {
	for {
		r := l.next()
		if r == 0 {
			break
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			l.backup()
			break
		}
	}
	return l.emit(tokenIdentifier)
}

func (l *sqlLexer) scanComment() token {
	if l.peek() == '*' {
		l.next() // consume *
		for {
			r := l.next()
			if r == 0 {
				break
			}
			if r == '*' && l.peek() == '/' {
				l.next() // consume /
				break
			}
		}
	} else {
		// Single line comment
		for {
			r := l.next()
			if r == 0 || r == '\n' {
				break
			}
		}
	}
	return l.emit(tokenComment)
}

func (l *sqlLexer) nextToken() token {
	if l.pos >= len(l.input) {
		return token{typ: tokenUnknown}
	}

	r := l.next()
	if r == 0 {
		return token{typ: tokenUnknown}
	}

	if unicode.IsSpace(r) {
		return l.skipWhitespace()
	}

	if unicode.IsDigit(r) || (r == '-' || r == '+') && unicode.IsDigit(l.peek()) {
		return l.scanNumber()
	}

	if r == '\'' || r == '"' || (r == '$' && l.peek() == '$') {
		return l.scanString()
	}

	if r == '/' && (l.peek() == '*' || l.peek() == '/') {
		return l.scanComment()
	}

	if unicode.IsLetter(r) || r == '_' {
		return l.scanIdentifier()
	}

	l.pos = l.start + 1
	return l.emit(tokenPunctuation)
}

type sqlParser struct {
	lexer      *sqlLexer
	operation  string
	parenLevel int
	inValues   bool
	inIN       bool
}

func (p *sqlParser) parseToken(tok token) string {
	if p.inValues {
		if tok.typ == tokenPunctuation && tok.value == ")" {
			p.inValues = false
			p.parenLevel--
			return ")"
		}
		return ""
	}

	switch tok.typ {
	case tokenWhitespace:
		return " "
	case tokenNumber:
		return "?"
	case tokenString:
		if p.operation == "" {
			upper := strings.ToUpper(tok.value)
			if isRedisCommand(upper) {
				p.operation = upper
				return tok.value
			}
		}
		return "?"
	case tokenComment:
		return tok.value
	case tokenIdentifier:
		upper := strings.ToUpper(tok.value)

		if p.operation == "" {
			if isRedisCommand(upper) {
				p.operation = upper
				return tok.value
			}
		}

		switch upper {
		case "SELECT", "INSERT", "UPDATE", "DELETE", "CREATE", "DROP", "ALTER", "MERGE":
			p.operation = upper
		case "IN":
			p.inIN = true
		}
		return tok.value
	case tokenPunctuation:
		switch tok.value {
		case "(":
			p.parenLevel++
			if p.inIN {
				p.inValues = true
				p.inIN = false
				return "(?"
			}
		case ")":
			p.parenLevel--
		}
		return tok.value
	default:
		return tok.value
	}
}

func isRedisCommand(cmd string) bool {
	switch cmd {
	case "SET", "GET", "HSET", "ZADD", "ZRANGEBYSCORE", "MULTI", "EXEC":
		return true
	default:
		return false
	}
}

func sanitizeDBStatement[K any](target ottl.StringGetter[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		str, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		if str == "" {
			return str, nil
		}

		lexer := &sqlLexer{input: str}
		parser := &sqlParser{lexer: lexer}
		var result strings.Builder

		for {
			tok := lexer.nextToken()
			if tok.typ == tokenUnknown {
				break
			}
			result.WriteString(parser.parseToken(tok))
		}

		normalized := strings.Join(strings.Fields(result.String()), " ")

		return normalized, nil
	}
}
