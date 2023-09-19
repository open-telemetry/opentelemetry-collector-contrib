package gojsonschemagenerator

import (
	"sort"
	"unicode"

	"github.com/atombender/go-jsonschema/pkg/codegen"
	"github.com/atombender/go-jsonschema/pkg/schemas"
)

func splitIdentifierByCaseAndSeparators(s string) []string {
	if len(s) == 0 {
		return nil
	}

	type state int

	const (
		stateNothing state = iota
		stateLower
		stateUpper
		stateNumber
		stateDelimiter
	)

	var result []string

	currState, j := stateNothing, 0

	for i := 0; i < len(s); i++ {
		var nextState state

		c := rune(s[i])

		switch {
		case unicode.IsLower(c):
			nextState = stateLower

		case unicode.IsUpper(c):
			nextState = stateUpper

		case unicode.IsNumber(c):
			nextState = stateNumber

		default:
			nextState = stateDelimiter
		}

		if nextState != currState {
			if currState == stateDelimiter {
				j = i
			} else if !(currState == stateUpper && nextState == stateLower) {
				if i > j {
					result = append(result, s[j:i])
				}
				j = i
			}

			currState = nextState
		}
	}

	if currState != stateDelimiter && len(s)-j > 0 {
		result = append(result, s[j:])
	}

	return result
}

func sortPropertiesByName(props map[string]*schemas.Type) []string {
	names := make([]string, 0, len(props))
	for name := range props {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}

func sortDefinitionsByName(defs schemas.Definitions) []string {
	names := make([]string, 0, len(defs))

	for name := range defs {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}

func isNamedType(t codegen.Type) bool {
	switch x := t.(type) {
	case *codegen.NamedType:
		return true

	case *codegen.PointerType:
		if _, ok := x.Type.(*codegen.NamedType); ok {
			return true
		}
	}

	return false
}
