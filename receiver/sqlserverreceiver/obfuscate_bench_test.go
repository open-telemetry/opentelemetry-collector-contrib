// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func BenchmarkObfuscateXMLPlanWithCache(b *testing.B) {
	benchmarkObfuscateXMLPlan(b, true)
}

func BenchmarkObfuscateXMLPlanWithoutCache(b *testing.B) {
	benchmarkObfuscateXMLPlan(b, false)
}

func benchmarkObfuscateXMLPlan(b *testing.B, cacheEnabled bool) {
	content, err := os.ReadFile(filepath.Join("testdata", "inputQueryPlan.xml"))
	if err != nil {
		b.Fatal(err)
	}
	plan := string(content)
	obfuscator := newObfuscator(zap.NewNop(), cacheEnabled)
	result, err := obfuscator.obfuscateXMLPlan(plan)
	if err != nil || result == "" {
		b.Fatalf("warm-up failed: result empty=%t, err=%v", result == "", err)
	}
	b.SetBytes(int64(len(plan)))
	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		result, err = obfuscator.obfuscateXMLPlan(plan)
		if err != nil {
			b.Fatal(err)
		}
		if result == "" {
			b.Fatal("obfuscated plan is empty")
		}
	}
}

func BenchmarkObfuscateXMLPlanWithCacheDistinct(b *testing.B) {
	content, err := os.ReadFile(filepath.Join("testdata", "inputQueryPlan.xml"))
	if err != nil {
		b.Fatal(err)
	}
	plan := string(content)
	plans := make([]string, xmlPlanCacheEntries+1)
	for i := range plans {
		plans[i] = fmt.Sprintf("%s<!-- plan %d -->", plan, i)
	}
	obfuscator := newObfuscator(zap.NewNop(), true)
	b.SetBytes(int64(len(plan)))
	b.ReportAllocs()

	for i := range b.N {
		result, err := obfuscator.obfuscateXMLPlan(plans[i%len(plans)])
		if err != nil {
			b.Fatal(err)
		}
		if result == "" {
			b.Fatal("obfuscated plan is empty")
		}
	}
}
