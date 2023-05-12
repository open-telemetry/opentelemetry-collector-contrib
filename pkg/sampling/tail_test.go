// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sampling

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func must[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}
	return t
}

func mustNot[T any](t T, err error) error {
	if err == nil {
		return fmt.Errorf("expected an error, got nil")
	}
	return err
}

func TestValidAdjustedCountToTvalue(t *testing.T) {
	require.Equal(t, "0", must(AdjustedCountToTvalue(0)))
	require.Equal(t, "1", must(AdjustedCountToTvalue(1)))
	require.Equal(t, "2", must(AdjustedCountToTvalue(2)))

	const largest uint64 = 0x1p+56
	require.Equal(t, "72057594037927936", must(AdjustedCountToTvalue(largest)))
	require.Equal(t, fmt.Sprint(largest-1), must(AdjustedCountToTvalue(largest-1)))
}

func TestInvalidAdjustedCountToTvalue(t *testing.T) {
	// Because unsigned, no too-small value.
	require.Error(t, mustNot(AdjustedCountToTvalue(0x1p56+1)))
	require.Error(t, mustNot(AdjustedCountToTvalue(math.MaxInt64)))
}

func TestValidProbabilityToTvalue(t *testing.T) {
	require.Equal(t, "0x1p-01", must(ProbabilityToTvalue(0.5, -1)))
	require.Equal(t, "0x1p-56", must(ProbabilityToTvalue(0x1p-56, -1)))
	require.Equal(t, "0x1.555p-02", must(ProbabilityToTvalue(1/3., 3)))
}

func TestInvalidProbabilityToTvalue(t *testing.T) {
	// Too small
	require.Error(t, mustNot(ProbabilityToTvalue(0x1p-57, -1)))
	require.Error(t, mustNot(ProbabilityToTvalue(0x1p-57, 0)))

	// Too big
	require.Error(t, mustNot(ProbabilityToTvalue(1.1, -1)))
	require.Error(t, mustNot(ProbabilityToTvalue(1.1, 0)))

	// Bad precision
	require.Error(t, mustNot(ProbabilityToTvalue(0.5, -3)))
	require.Error(t, mustNot(ProbabilityToTvalue(0.5, 15)))
}

func testTValueToProb(tv string) (float64, error) {
	p, _, err := TvalueToProbabilityAndAdjustedCount(tv)
	return p, err
}

func testTValueToAdjCount(tv string) (float64, error) {
	_, ac, err := TvalueToProbabilityAndAdjustedCount(tv)
	return ac, err
}

func TestTvalueToProbability(t *testing.T) {
	require.Equal(t, 0.5, must(testTValueToProb("0.5")))
	require.Equal(t, 0.444, must(testTValueToProb("0.444")))
	require.Equal(t, 1.0, must(testTValueToProb("1")))
	require.Equal(t, 0.0, must(testTValueToProb("0")))

	require.InEpsilon(t, 1/3., must(testTValueToProb("3")), 1e-9)
}

func TestTvalueToAdjCount(t *testing.T) {
	require.Equal(t, 2.0, must(testTValueToAdjCount("0.5")))
	require.Equal(t, 2.0, must(testTValueToAdjCount("2")))
	require.Equal(t, 3., must(testTValueToAdjCount("3")))
	require.Equal(t, 5., must(testTValueToAdjCount("5")))

	require.InEpsilon(t, 1/0.444, must(testTValueToAdjCount("0.444")), 1e-9)
	require.InEpsilon(t, 1/0.111111, must(testTValueToAdjCount("0.111111")), 1e-9)

	require.Equal(t, 1.0, must(testTValueToAdjCount("1")))
	require.Equal(t, 0.0, must(testTValueToAdjCount("0")))
}

func TestProbabilityToThreshold(t *testing.T) {
	require.Equal(t,
		Threshold{0x7f, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		must(ProbabilityToThreshold(0.5)))
	require.Equal(t,
		Threshold{0, 0, 0, 0, 0, 0, 0},
		must(ProbabilityToThreshold(0x1p-56)))
	require.Equal(t,
		Threshold{0, 0, 0, 0, 0, 0, 0xff},
		must(ProbabilityToThreshold(0x100p-56)))
	require.Equal(t,
		Threshold{0, 0, 0, 0, 0, 0, 0x01},
		must(ProbabilityToThreshold(0x1p-55)))
	require.Equal(t,
		Threshold{0, 0, 0, 0, 0, 0, 0x01},
		must(ProbabilityToThreshold(0x1p-55)))
	require.Equal(t,
		Threshold{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		must(ProbabilityToThreshold(1.0)))

	require.Equal(t,
		Threshold{0x55, 0x53, 0xff, 0xff, 0xff, 0xff, 0xff},
		must(ProbabilityToThreshold(0x1.555p-2)))
}
