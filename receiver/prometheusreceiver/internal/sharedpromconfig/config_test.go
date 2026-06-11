// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sharedpromconfig

import (
	"sync"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	promconfig "github.com/prometheus/prometheus/config"
	"github.com/stretchr/testify/assert"
)

func TestGetReturnsShallowCopy(t *testing.T) {
	baseCfg := promconfig.DefaultConfig
	sc := NewConfig(&baseCfg)

	got := sc.Get()
	assert.Equal(t, baseCfg, got)

	// Mutating the returned value must not affect the original.
	got.GlobalConfig.ScrapeInterval = model.Duration(99 * time.Second)
	original := sc.Get()
	assert.NotEqual(t, got.GlobalConfig.ScrapeInterval, original.GlobalConfig.ScrapeInterval)
}

func TestMutateModifiesUnderlyingConfig(t *testing.T) {
	baseCfg := promconfig.DefaultConfig
	sc := NewConfig(&baseCfg)

	newInterval := model.Duration(42 * time.Second)
	sc.Mutate(func(cfg *promconfig.Config) {
		cfg.GlobalConfig.ScrapeInterval = newInterval
	})

	got := sc.Get()
	assert.Equal(t, newInterval, got.GlobalConfig.ScrapeInterval)
}

func TestConcurrentAccess(t *testing.T) {
	baseCfg := promconfig.DefaultConfig
	sc := NewConfig(&baseCfg)

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	for range goroutines {
		go func() {
			defer wg.Done()
			promCfg := sc.Get()
			assert.NotNil(t, &promCfg)
		}()
	}

	for i := range goroutines {
		go func() {
			defer wg.Done()
			sc.Mutate(func(cfg *promconfig.Config) {
				cfg.GlobalConfig.ScrapeInterval = model.Duration(time.Duration(i) * time.Second)
			})
		}()
	}

	wg.Wait()
}
