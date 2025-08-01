// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/AlessandroPomponio/go-gibberish/gibberish"
	"github.com/AlessandroPomponio/go-gibberish/structs"
	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

var classifier *structs.GibberishData

const maxSegments = 10

var words *lru.Cache[string, bool]

//go:embed func_sanitize_url_classifier.json
var dataFile embed.FS

type SanitizeURLArguments[K any] struct {
	Target      ottl.StringGetter[K]
	Replacement ottl.StringGetter[K]
}

func NewSanitizeURLFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("SanitizeURL", &SanitizeURLArguments[K]{}, createSanitizeURLFunction[K])
}

func createSanitizeURLFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*SanitizeURLArguments[K])

	if !ok {
		return nil, errors.New("SanitizeURLFactory args must be of type *SanitizeURLArguments[K]")
	}

	return SanitizeURL(args.Target, args.Replacement)
}

func SanitizeURL[K any](target, replacement ottl.StringGetter[K]) (ottl.ExprFunc[K], error) {
	if classifier == nil {
		if err := initAutoClassifier(); err != nil {
			return nil, fmt.Errorf("failed to initialize classifier: %w", err)
		}
	}

	return func(ctx context.Context, tCtx K) (any, error) {
		val, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		replacementVal, err := replacement.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		return clusterPath(val, replacementVal), nil
	}, nil
}

func initAutoClassifier() error {
	content, err := dataFile.ReadFile("func_sanitize_url_classifier.json")
	if err != nil {
		return fmt.Errorf("LoadKnowledgeBase: unable to read knowledge base content: %w", err)
	}

	err = json.Unmarshal(content, &classifier)
	if err != nil {
		return fmt.Errorf("LoadKnowledgeBase: unable to unmarshal knowledge base content: %w", err)
	}

	words, err = lru.New[string, bool](8192)
	if err != nil {
		return fmt.Errorf("LoadKnowledgeBase: unable to create LRU cache: %w", err)
	}

	return nil
}

// clusterPath is a function that clusters a path by replacing segments with a replacement character.
// Based on:
// https://github.com/open-telemetry/opentelemetry-ebpf-instrumentation/blob/38ca7938595409b8ffe6b897c14a0e3280dd2941/pkg/components/transform/route/cluster.go#L48
func clusterPath(path, replacement string) string {
	if path == "" {
		return path
	}

	pathBytes := []byte(path)
	replacementBytes := []byte(replacement)
	result := make([]byte, 0, len(pathBytes)+len(replacementBytes)*10)

	skip := false
	skipGrace := true
	nSegments := 0
	segmentStart := 0

	for i, c := range pathBytes {
		if c == '/' {
			nSegments++
			if skip {
				result = append(result, replacementBytes...)
			} else if i > segmentStart {
				segment := string(pathBytes[segmentStart:i])
				if !okWord(segment) {
					result = append(result, replacementBytes...)
				} else {
					result = append(result, pathBytes[segmentStart:i]...)
				}
			}

			if nSegments >= maxSegments {
				break
			}

			result = append(result, '/')
			segmentStart = i + 1
			skip = false
			skipGrace = true
		} else if !skip {
			if !isAlpha(c) {
				if skipGrace && (i-segmentStart) == 1 {
					skipGrace = false
					continue
				}
				skip = true
			}
		}
	}

	if segmentStart < len(pathBytes) {
		if skip {
			result = append(result, replacementBytes...)
		} else {
			segment := string(pathBytes[segmentStart:])
			if !okWord(segment) {
				result = append(result, replacementBytes...)
			} else {
				result = append(result, pathBytes[segmentStart:]...)
			}
		}
	}

	return string(result)
}

func okWord(w string) bool {
	_, ok := words.Get(w)
	if ok {
		return ok
	}
	if gibberish.IsGibberish(w, classifier) {
		return false
	}

	words.Add(w, true)
	return true
}

func isAlpha(c byte) bool {
	switch c {
	case '-', '_', '.':
		return true
	}
	return (c|0x20) >= 'a' && (c|0x20) <= 'z'
}
