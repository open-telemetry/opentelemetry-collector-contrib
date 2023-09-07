// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package probabilisticsamplerprocessor

import "fmt"

type SamplerMode string

const (
	HashSeed     SamplerMode = "hash_seed"
	Equalizing   SamplerMode = "equalizing"
	Proportional SamplerMode = "proportional"
	modeUnset    SamplerMode = ""

	DefaultMode = Proportional
)

func (sm *SamplerMode) UnmarshalText(in []byte) error {
	switch mode := SamplerMode(in); mode {
	case HashSeed,
		Equalizing,
		Proportional,
		modeUnset:
		*sm = mode
		return nil
	default:
		return fmt.Errorf("unsupported sampler mode %q", mode)
	}
}
