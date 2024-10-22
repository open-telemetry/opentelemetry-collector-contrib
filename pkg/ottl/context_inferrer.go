// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"

var (
	defaultContextInferPriority = []string{
		"log",
		"metric",
		"datapoint",
		"spanevent",
		"span",
		"resource",
		"scope",
		"instrumentation_scope",
	}
)

type ContextInferrer interface {
	Infer(statements []string) (string, error)
}

type staticContextInferrer struct {
	context string
}

func (s *staticContextInferrer) Infer(_ []string) (string, error) {
	return s.context, nil
}

func NewStaticContextInferrer(context string) ContextInferrer {
	return &staticContextInferrer{context: context}
}

type defaultContextInferrer struct {
	contextPriority      map[string]int
	ignoreUnknownContext bool
}

func (s *defaultContextInferrer) Infer(statements []string) (string, error) {
	var inferredContext string
	var inferredContextPriority int

	for _, statement := range statements {
		parsed, err := parseStatement(statement)
		if err != nil {
			return inferredContext, err
		}

		for _, p := range getParsedStatementPaths(parsed) {
			pathContextPriority, ok := s.contextPriority[p.Context]
			if !ok {
				if s.ignoreUnknownContext {
					continue
				}
				pathContextPriority = len(s.contextPriority) // Lowest priority
			}

			if inferredContext == "" || pathContextPriority < inferredContextPriority {
				inferredContext = p.Context
				inferredContextPriority = pathContextPriority
			}
		}
	}

	return inferredContext, nil
}

func NewDefaultContextInferrer() ContextInferrer {
	return NewContextInferrerWithPriority(defaultContextInferPriority, false)
}

func NewContextInferrerWithPriority(contextsPriority []string, ignoreUnknownContext bool) ContextInferrer {
	contextPriority := make(map[string]int, len(contextsPriority))
	for i, ctx := range contextsPriority {
		contextPriority[ctx] = i
	}
	return &defaultContextInferrer{
		contextPriority:      contextPriority,
		ignoreUnknownContext: ignoreUnknownContext,
	}
}
