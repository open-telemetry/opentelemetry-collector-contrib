// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package intracesampler // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/intracesamplerprocessor"

import (
	"hash/fnv"
)

// computeHash creates a hash using the FNV-1a algorithm
func computeHash(b []byte, seed []byte) uint32 {
	hash := fnv.New32a()
	// the implementation fnv.Write() does not return an error, see hash/fnv/fnv.go
	_, _ = hash.Write(seed)
	_, _ = hash.Write(b)
	return hash.Sum32()
}

// i32tob converts a seed to a byte array to be used as part of fnv.Write()
// this code is copied from probability sampler for them to apply the same hash
// and achieve consistent sampling results
// https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/e15755144d6ac15689ef04429465da063a001302/processor/probabilisticsamplerprocessor/fnvhasher.go#L31
// it can potentially be moved to a common place which both samplers consume
func i32tob(val uint32) []byte {
	r := make([]byte, 4)
	for i := uint32(0); i < 4; i++ {
		r[i] = byte((val >> (8 * i)) & 0xff)
	}
	return r
}
