// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package regexp

import (
	"regexp"

	"github.com/wasilibs/go-re2"
)

type Matcher interface {
	MatchString(s string) bool
	FindAllString(s string, n int) []string
	ReplaceAllString(s string, replacement string) string
	SubexpNames() []string
	FindStringSubmatch(s string) []string
	FindAllStringSubmatchIndex(s string, n int) [][]int
	ExpandString(dst []byte, template string, src string, match []int) []byte
}

const patternLengthThreshold = 200 // the threshold is chosen based on the performance of the regex engine

func Compile(pattern string) (Matcher, error) {
	if len(pattern) >= patternLengthThreshold {
		return re2.Compile(pattern)
	}
	return regexp.Compile(pattern)
}

func MustCompile(pattern string) Matcher {
	if len(pattern) >= patternLengthThreshold {
		return re2.MustCompile(pattern)
	}
	return regexp.MustCompile(pattern)
}
