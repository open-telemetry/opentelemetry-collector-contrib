// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package windows // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/windows"

import (
	"strings"
)

func parseSecurity(message string) (string, map[string]any) {
	var subject string
	details := map[string]any{}

	mp := newMessageProcessor(message)

	// First line is expected to be the first return value
	l := mp.next()
	switch l.t {
	case valueType:
		subject = l.v
	case keyType:
		subject = l.k
	case pairType, emptyType:
		return message, nil
	}

	var moreInfo []string

	for mp.hasNext() {
		l = mp.next()
		switch l.t {
		case valueType:
			moreInfo = append(moreInfo, l.v)
		case keyType:
			if !mp.hasNextIndented(l.i + 1) {
				// standalone key/value pair with empty value
				details[l.k] = "-"
				continue
			}
			details[l.k] = mp.consumeSubsection(l.i + 1)
		case pairType:
			if !mp.hasNextIndented(l.i + 1) {
				// standalone key/value pair
				details[l.k] = l.v
				continue
			}
			// value was first in a list
			details[l.k] = append([]string{l.v}, mp.consumeSublist(l.i+1)...)
		case emptyType:
			continue
		}
	}

	if len(moreInfo) > 0 {
		details["Additional Context"] = moreInfo
	}

	return subject, details
}

func (mp *messageProcessor) consumeSubsection(depth int) map[string]any {
	sub := map[string]any{}
	for mp.hasNext() {
		l := mp.next()
		switch l.t {
		case emptyType:
			return sub
		case pairType:
			sub[l.k] = l.v
		case keyType:
			if !mp.hasNextIndented(depth + 1) {
				// standalone key/value pair with missing value
				sub[l.k] = "-"
				continue
			}
			sub[l.k] = mp.consumeSublist(depth + 1)
		case valueType:
			continue
		}
	}
	return sub
}

func (mp *messageProcessor) consumeSublist(depth int) []string {
	var sublist []string
	for mp.hasNext() {
		if !mp.hasNextIndented(depth) {
			return sublist
		}
		l := mp.next()
		switch l.t {
		case valueType:
			sublist = append(sublist, l.v)
		case keyType: // not expected, but handle
			sublist = append(sublist, l.k)
		case pairType, emptyType:
			// not expected
		}
	}
	return sublist
}

type messageProcessor struct {
	lines []*parsedLine
	ptr   int
}

type parsedLine struct {
	t lineType
	i int
	k string
	v string
}

type lineType int

const (
	emptyType lineType = iota
	keyType
	valueType
	pairType
)

func newMessageProcessor(message string) *messageProcessor {
	unparsedLines := strings.Split(strings.TrimSpace(message), "\n")
	parsedLines := make([]*parsedLine, len(unparsedLines))
	for i, unparsedLine := range unparsedLines {
		parsedLines[i] = parse(unparsedLine)
	}
	return &messageProcessor{lines: parsedLines}
}

func parse(line string) *parsedLine {
	i := countIndent(line)
	l := strings.TrimSpace(line)
	if l == "" {
		return &parsedLine{t: emptyType, i: i}
	}

	if strings.Contains(l, ":\t") {
		k, v := parseKeyValue(l)
		return &parsedLine{t: pairType, i: i, k: k, v: v}
	}

	if strings.HasSuffix(l, ":") {
		return &parsedLine{t: keyType, i: i, k: l[:len(l)-1]}
	}

	return &parsedLine{t: valueType, i: i, v: l}
}

// return next line and increment position
func (mp *messageProcessor) next() *parsedLine {
	defer mp.step()
	return mp.lines[mp.ptr]
}

// return next line but do not increment position
func (mp *messageProcessor) peek() *parsedLine {
	return mp.lines[mp.ptr]
}

// just increment position
func (mp *messageProcessor) step() {
	mp.ptr++
}

func (mp *messageProcessor) hasNext() bool {
	return mp.ptr < len(mp.lines)
}

func (mp *messageProcessor) hasNextIndented(minDepth int) bool {
	if !mp.hasNext() || mp.ptr == 0 {
		return false
	}

	l := mp.peek()
	if l.t == emptyType {
		return false
	}

	return l.i >= minDepth
}

func countIndent(line string) int {
	i := 1
	for pre := strings.Repeat("\t", i); strings.HasPrefix(line, pre); pre = strings.Repeat("\t", i) {
		i++
	}
	return i - 1
}

func parseKeyValue(line string) (string, string) {
	kv := strings.SplitN(line, ":\t", 2)
	return strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1])
}
