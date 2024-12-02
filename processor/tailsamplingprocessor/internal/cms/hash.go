// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cms // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/cms"

import (
	"github.com/cespare/xxhash/v2"
)

// Random generated seeds for xxHash
var hashSeeds = []uint64{
	11595015838869646110,
	6812085131706188016,
	1803417234240274060,
	9910387706731171320,
	6192372884267156726,
	9359297034374090798,
	10699310435244823924,
	2694511717751745834,
	381360183478099046,
	3813652365689143046,
	2745863731201646884,
	9338067737075496131,
	2310364619108435937,
	2033119307453415722,
	18154805337904800280,
	16153398036464115640,
	11576467370134829350,
	16139083825458188821,
	14613404529444667025,
	8229605496796251508,
	14043697971178212370,
	5104099633310233611,
	8840567979630932215,
	3619854489427682144,
	2922888160084146262,
	1066417268237148873,
	10391653214809458763,
	5008947111593455631,
	2544378244597710161,
	1282165131157204414,
	15346189051374937777,
	8983218487838504684,
}

type XXHasher struct {
	// h xxHash implementation of hash.Hash64
	h *xxhash.Digest
	// seedIdx seed serial index
	seedIdx int
	// maxVal max value that hasher could produce
	maxVal uint32
}

func NewHWHasher(length uint32, idx int) *XXHasher {
	h := xxhash.NewWithSeed(hashSeeds[idx])
	return &XXHasher{
		h:       h,
		maxVal:  length,
		seedIdx: idx,
	}
}

func (hw *XXHasher) Hash(data []byte) uint32 {
	hw.h.ResetWithSeed(hashSeeds[hw.seedIdx])
	_, _ = hw.h.Write(data)

	return uint32(hw.h.Sum64() % uint64(hw.maxVal))
}
