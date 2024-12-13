// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"

import "math"

var defaultContextInferPriority = []string{
	"log",
	"metric",
	"datapoint",
	"spanevent",
	"span",
	"resource",
	"scope",
	"instrumentation_scope",
}

// contextInferrer is an interface used to infer the OTTL context from statements paths.
type contextInferrer interface {
	// infer returns the OTTL context inferred from the given statements paths.
	infer(statements []string) (string, error)
}

type priorityContextInferrer struct {
	contextPriority map[string]int
}

func (s *priorityContextInferrer) infer(statements []string) (string, error) {
	var inferredContext string
	var inferredContextPriority int

	for _, statement := range statements {
		parsed, err := parseStatement(statement)
		if err != nil {
			return inferredContext, err
		}

		for _, p := range getParsedStatementPaths(parsed) {
			pathContext := s.getContextCandidate(p)
			pathContextPriority, ok := s.contextPriority[pathContext]
			if !ok {
				// Lowest priority
				pathContextPriority = math.MaxInt
			}

			if inferredContext == "" || pathContextPriority < inferredContextPriority {
				inferredContext = pathContext
				inferredContextPriority = pathContextPriority
			}
		}
	}

	return inferredContext, nil
}

// When a path has no dots separators (e.g.: resource), the grammar extracts it into the
// path.Fields slice, letting the path.Context empty. This function returns either the
// path.Context string or, if it's eligible and meets certain conditions, the first
// path.Fields name.
func (s *priorityContextInferrer) getContextCandidate(p path) string {
	if p.Context != "" {
		return p.Context
	}
	// If it has multiple fields or keys, it means the path has at least one dot on it,
	// and isn't a context access.
	if len(p.Fields) != 1 || len(p.Fields[0].Keys) > 0 {
		return ""
	}
	_, ok := s.contextPriority[p.Fields[0].Name]
	if ok {
		return p.Fields[0].Name
	}
	return ""
}

// defaultPriorityContextInferrer is like newPriorityContextInferrer, but using the default
// context priorities and ignoring unknown/non-prioritized contexts.
func defaultPriorityContextInferrer() contextInferrer {
	return newPriorityContextInferrer(defaultContextInferPriority)
}

// newPriorityContextInferrer creates a new priority-based context inferrer.
// To infer the context, it compares all [ottl.Path.Context] values, prioritizing them based
// on the provide contextsPriority argument, the lower the context position is in the array,
// the more priority it will have over other items.
// If unknown/non-prioritized contexts are found on the statements, they can be either ignored
// or considered when no other prioritized context is found. To skip unknown contexts, the
// ignoreUnknownContext argument must be set to false.
func newPriorityContextInferrer(contextsPriority []string) contextInferrer {
	contextPriority := make(map[string]int, len(contextsPriority))
	for i, ctx := range contextsPriority {
		contextPriority[ctx] = i
	}
	return &priorityContextInferrer{
		contextPriority: contextPriority,
	}
}
