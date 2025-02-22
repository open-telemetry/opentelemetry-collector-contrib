// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package routingconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector"

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"go.opentelemetry.io/collector/client"
	"google.golang.org/grpc/metadata"
)

// This file defines an extremely simple request condition grammar. The goal is to provide a similar feel to OTTL,
// but it's not clear that anything more than a simple comparison is needed.  We can expand this grammar in the
// future if needed. For now, it expects the condition to be in exactly the format:
// 'request["<name>"] <comparator> <value>' where <comparator> is either '==' or '!='.

var (
	requestFieldRegex = regexp.MustCompile(`request\[".*"\]`)
	valueFieldRegex   = regexp.MustCompile(`".*"`)
	comparatorRegex   = regexp.MustCompile(`==|!=`)
)

type requestCondition struct {
	compareFunc   func(string) bool
	attributeName string
}

func parseRequestCondition(condition string) (*requestCondition, error) {
	if condition == "" {
		return nil, errors.New("condition is empty")
	}

	comparators := comparatorRegex.FindAllString(condition, 2)
	switch {
	case len(comparators) == 0:
		return nil, errors.New("condition does not contain a valid comparator")
	case len(comparators) > 1:
		return nil, errors.New("condition contains multiple comparators")
	}

	parts := strings.Split(condition, comparators[0])
	if len(parts) < 2 {
		return nil, errors.New("condition does not contain a valid comparator")
	}
	if len(parts) > 2 {
		return nil, errors.New("condition contains multiple comparators")
	}
	parts[0] = strings.TrimSpace(parts[0])
	parts[1] = strings.TrimSpace(parts[1])

	if !requestFieldRegex.MatchString(parts[0]) {
		return nil, errors.New(`condition must have format 'request["<name>"] <comparator> <value>'`)
	}
	if !valueFieldRegex.MatchString(parts[1]) {
		return nil, errors.New(`condition must have format 'request["<name>"] <comparator> "<value>"'`)
	}
	valueWithoutQuotes := strings.TrimSuffix(strings.TrimPrefix(parts[1], `"`), `"`)

	compareFunc := func(value string) bool {
		return value == valueWithoutQuotes
	}
	if comparators[0] == "!=" {
		compareFunc = func(value string) bool {
			return value != valueWithoutQuotes
		}
	}

	return &requestCondition{
		attributeName: strings.TrimSuffix(strings.TrimPrefix(parts[0], `request["`), `"]`),
		compareFunc:   compareFunc,
	}, nil
}

func (rc *requestCondition) matchRequest(ctx context.Context) bool {
	return rc.matchGRPC(ctx) || rc.matchHTTP(ctx)
}

func (rc *requestCondition) matchGRPC(ctx context.Context) bool {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return false
	}
	values, ok := md[strings.ToLower(rc.attributeName)]
	if !ok {
		return false
	}
	for _, value := range values {
		if rc.compareFunc(value) {
			return true
		}
	}
	return false
}

func (rc *requestCondition) matchHTTP(ctx context.Context) bool {
	values := client.FromContext(ctx).Metadata.Get(rc.attributeName)
	for _, value := range values {
		if rc.compareFunc(value) {
			return true
		}
	}
	return false
}
