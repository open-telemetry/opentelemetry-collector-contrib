// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cms // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/cms"

import "fmt"

func countOfInsertions(n int) int {
	return (n / 2) * (1 + n)
}

func makeTestCMSKey(n int) []byte {
	return []byte(fmt.Sprintf("%d", n))
}

type partialVisitor[T any] struct {
	currIdx    int
	maxNumOfEl int
	visited    []T
}

func newPVisitor[T any](maxNumOfEl int) *partialVisitor[T] {
	return &partialVisitor[T]{
		maxNumOfEl: maxNumOfEl,
		visited:    make([]T, 0),
	}
}

func (p *partialVisitor[T]) visit(val T) bool {
	if p.currIdx >= p.maxNumOfEl {
		return false
	}
	p.visited = append(p.visited, val)
	p.currIdx++
	return true
}

type CntMap map[string]uint32

type StubCms struct {
	InsertionsReq         int
	InsertionsWithCnt     int
	InsertionsWithCntKeys []string
	CountReq              int
	ClearCnt              int
	countResponses        CntMap
	id                    int
}

func CopyCmsStubSlice(src []CountMinSketch) []CountMinSketch {
	dst := make([]CountMinSketch, 0, len(src))
	for _, cs := range src {
		s := *(cs.(*StubCms))
		dst = append(dst, &s)
	}
	return dst
}

func NewEmptyCmsStub(id int) *StubCms {
	return &StubCms{
		id:                    id,
		InsertionsWithCntKeys: make([]string, 0),
	}
}

func NewCmsStubWithCounts(id int, counts CntMap) *StubCms {
	return &StubCms{
		countResponses: counts,
		id:             id,
	}
}

func (s *StubCms) InsertWithCount(element []byte) uint32 {
	s.InsertionsWithCnt++
	s.InsertionsWithCntKeys = append(s.InsertionsWithCntKeys, string(element))
	return s.countResponses[string(element)]
}

func (s *StubCms) Count(element []byte) uint32 {
	s.CountReq++
	return s.countResponses[string(element)]
}

func (s *StubCms) Insert([]byte) {
	s.InsertionsReq++
}

func (s *StubCms) Clear() {
	s.ClearCnt++
}
