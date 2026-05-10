// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package regexp // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/regexp"

import (
	"regexp"

	re2 "github.com/wasilibs/go-re2"
)

type Matcher interface {
	String() string
	MatchString(s string) bool
	FindAllString(s string, n int) []string
	ReplaceAllString(s, replacement string) string
	SubexpNames() []string
	FindStringSubmatch(s string) []string
	FindAllStringSubmatchIndex(s string, n int) [][]int
	ExpandString(dst []byte, template, src string, match []int) []byte
}

const patternLengthThreshold = 200

var compileRE2 = re2.Compile

func Compile(pattern string) (Matcher, error) {
	if len(pattern) >= patternLengthThreshold {
		matcher, err := compileRE2(pattern)
		if err == nil {
			return matcher, nil
		}
	}
	return regexp.Compile(pattern)
}

func MustCompile(pattern string) Matcher {
	matcher, err := Compile(pattern)
	if err != nil {
		panic(err)
	}
	return matcher
}
