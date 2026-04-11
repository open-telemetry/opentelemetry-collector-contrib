// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package profiles

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"fmt"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/config"
	types "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/pkg"
)

type worker struct {
	running         *atomic.Bool          // pointer to shared flag that indicates it's time to stop the test
	numProfiles     int                   // how many profiles the worker has to generate (only when duration==0)
	sampleCount     int                   // number of samples per profile
	stackDepth      int                   // maximum number of frames per stack
	uniqueFunctions int                   // number of distinct functions in the profile dictionary
	profileDuration time.Duration         // duration represented by each profile (DurationNano)
	totalDuration   types.DurationWithInf // how long to run the test for (overrides `numProfiles`)
	limitPerSecond  rate.Limit            // how many profiles per second to generate
	wg              *sync.WaitGroup       // notify when done
	logger          *zap.Logger           // logger
	index           int                   // worker index
	traceID         string                // traceID for profile-to-trace correlation
	spanID          string                // spanID for profile-to-trace correlation
	batch           bool                  // whether to batch profiles
	batchBuffer     *pprofile.Profiles    // buffer for batching profiles into a single export request
	bufferCount     int                   // number of profile records in the buffer
	bufferMutex     sync.Mutex            // mutex for thread-safe access to buffer
	batchSize       int                   // number of profiles to batch before flushing
	loadSize        int                   // desired minimum size in MB of string data for each generated profile
	allowFailures   bool                  // whether to continue on export failures
	rand            *rand.Rand            // seeded PRNG for sample values
}

func (w *worker) simulateProfiles(res pcommon.Map, exporter profileExporter, telemetryAttributes []attribute.KeyValue) {
	limiter := rate.NewLimiter(w.limitPerSecond, 1)
	var i int64

	for w.running.Load() {
		if err := limiter.Wait(context.Background()); err != nil {
			w.logger.Fatal("limiter wait failed, retry", zap.Error(err))
		}

		if w.batch {
			if err := w.addToBuffer(res, telemetryAttributes, exporter); err != nil {
				w.logger.Fatal("failed to generate profile", zap.Error(err))
			}
		} else {
			td := pprofile.NewProfiles()
			if err := w.generateProfile(td, res, telemetryAttributes); err != nil {
				w.logger.Fatal("failed to generate profile", zap.Error(err))
			}
			if err := exporter.Export(context.Background(), td); err != nil {
				if w.allowFailures {
					w.logger.Error("exporter failed, continuing due to --allow-export-failures", zap.Error(err))
				} else {
					w.logger.Fatal("exporter failed", zap.Error(err))
				}
			}
		}

		i++
		if w.numProfiles != 0 && i >= int64(w.numProfiles) {
			break
		}
	}

	// Flush any remaining profiles in the buffer
	if w.batch {
		w.flushBuffer(exporter)
	}

	w.logger.Info("profiles generated", zap.Int64("profiles", i))
	w.wg.Done()
}

// seedDictionaryZeroValues ensures index 0 of every shared dictionary table holds
// a zero value, as required by the OTLP profiles specification. The Set* helpers
// deduplicate, so this is idempotent and safe to call for every profile that shares
// a dictionary (e.g. when batching).
func seedDictionaryZeroValues(dict pprofile.ProfilesDictionary, st pcommon.StringSlice) {
	_, _ = pprofile.SetString(st, "")
	_, _ = pprofile.SetMapping(dict.MappingTable(), pprofile.NewMapping())
	_, _ = pprofile.SetFunction(dict.FunctionTable(), pprofile.NewFunction())
	_, _ = pprofile.SetLocation(dict.LocationTable(), pprofile.NewLocation())
	_, _ = pprofile.SetStack(dict.StackTable(), pprofile.NewStack())
	_, _ = pprofile.SetLink(dict.LinkTable(), pprofile.NewLink())
	_, _ = pprofile.SetAttribute(dict.AttributeTable(), pprofile.NewKeyValueAndUnit())
}

func (w *worker) generateProfile(td pprofile.Profiles, res pcommon.Map, telemetryAttributes []attribute.KeyValue) error {
	dict := td.Dictionary()
	st := dict.StringTable()

	// The OTLP profiles spec reserves index 0 of every shared dictionary table
	// for a zero value; real entries are referenced by their returned indices.
	// Seeding keeps generated profiles conformant and is idempotent, so it is
	// safe to call for every profile sharing a dictionary (e.g. when batching).
	seedDictionaryZeroValues(dict, st)

	cpuIdx, _ := pprofile.SetString(st, "cpu")
	nsIdx, _ := pprofile.SetString(st, "nanoseconds")
	fileIdx, _ := pprofile.SetString(st, "synthetic.go")

	funcIndices := make([]int32, w.uniqueFunctions)
	for f := 0; f < w.uniqueFunctions; f++ {
		nameIdx, _ := pprofile.SetString(st, fmt.Sprintf("func_%d", f))
		fn := pprofile.NewFunction()
		fn.SetNameStrindex(nameIdx)
		fn.SetSystemNameStrindex(nameIdx)
		fn.SetFilenameStrindex(fileIdx)
		fn.SetStartLine(int64(f * 10))
		idx, _ := pprofile.SetFunction(dict.FunctionTable(), fn)
		funcIndices[f] = idx
	}

	mapping := pprofile.NewMapping()
	mapping.SetMemoryStart(0x400000)
	mapping.SetMemoryLimit(0x800000)
	mapping.SetFilenameStrindex(fileIdx)
	mappingIdx, _ := pprofile.SetMapping(dict.MappingTable(), mapping)

	locIndices := make([]int32, w.uniqueFunctions)
	for f := 0; f < w.uniqueFunctions; f++ {
		loc := pprofile.NewLocation()
		loc.SetMappingIndex(mappingIdx)
		loc.SetAddress(uint64(0x400000 + f*0x100))
		line := loc.Lines().AppendEmpty()
		line.SetFunctionIndex(funcIndices[f])
		line.SetLine(int64(f*10 + 1))
		idx, _ := pprofile.SetLocation(dict.LocationTable(), loc)
		locIndices[f] = idx
	}

	stackIndices := make([]int32, w.sampleCount)
	for s := 0; s < w.sampleCount; s++ {
		stack := pprofile.NewStack()
		for d := 0; d < w.stackDepth; d++ {
			funcIdx := (s + d) % w.uniqueFunctions
			stack.LocationIndices().Append(locIndices[funcIdx])
		}
		idx, _ := pprofile.SetStack(dict.StackTable(), stack)
		stackIndices[s] = idx
	}

	var linkIndex int32
	if w.traceID != "" || w.spanID != "" {
		realLink := pprofile.NewLink()
		if w.traceID != "" {
			traceIDBytes, _ := hex.DecodeString(w.traceID)
			var tid pcommon.TraceID
			copy(tid[:], traceIDBytes)
			realLink.SetTraceID(tid)
		}

		if w.spanID != "" {
			spanIDBytes, _ := hex.DecodeString(w.spanID)
			var sid pcommon.SpanID
			copy(sid[:], spanIDBytes)
			realLink.SetSpanID(sid)
		}

		idx, err := pprofile.SetLink(dict.LinkTable(), realLink)
		if err != nil {
			return fmt.Errorf("failed to set trace correlation link: %w", err)
		}
		linkIndex = idx
	}

	var sp pprofile.ScopeProfiles
	if td.ResourceProfiles().Len() > 0 {
		sp = td.ResourceProfiles().At(0).ScopeProfiles().At(0)
	} else {
		rp := td.ResourceProfiles().AppendEmpty()
		res.CopyTo(rp.Resource().Attributes())
		sp = rp.ScopeProfiles().AppendEmpty()
		sp.Scope().SetName("telemetrygen")
	}
	profile := sp.Profiles().AppendEmpty()

	var profileID pprofile.ProfileID
	if _, err := crand.Read(profileID[:]); err != nil {
		return fmt.Errorf("failed to generate profile ID: %w", err)
	}
	profile.SetProfileID(profileID)

	profile.SetTime(pcommon.NewTimestampFromTime(time.Now()))
	profile.SetDurationNano(uint64(w.profileDuration.Nanoseconds()))

	profile.SampleType().SetTypeStrindex(cpuIdx)
	profile.SampleType().SetUnitStrindex(nsIdx)

	profile.PeriodType().SetTypeStrindex(cpuIdx)
	profile.PeriodType().SetUnitStrindex(nsIdx)
	profile.SetPeriod(10000000) // 10ms in nanoseconds

	for _, attr := range telemetryAttributes {
		keyIdx, _ := pprofile.SetString(st, string(attr.Key))
		kvu := pprofile.NewKeyValueAndUnit()
		kvu.SetKeyStrindex(keyIdx)
		setProfileAttributeValue(kvu.Value(), attr.Value)
		attrIdx, _ := pprofile.SetAttribute(dict.AttributeTable(), kvu)
		profile.AttributeIndices().Append(attrIdx)
	}

	if w.loadSize > 0 {
		for j := 0; j < w.loadSize; j++ {
			loadKeyIdx, _ := pprofile.SetString(st, fmt.Sprintf("load-%d", j))
			kvu := pprofile.NewKeyValueAndUnit()
			kvu.SetKeyStrindex(loadKeyIdx)
			kvu.Value().SetStr(string(make([]byte, config.CharactersPerMB)))
			attrIdx, _ := pprofile.SetAttribute(dict.AttributeTable(), kvu)
			profile.AttributeIndices().Append(attrIdx)
		}
	}

	for s := 0; s < w.sampleCount; s++ {
		sample := profile.Samples().AppendEmpty()
		sample.SetStackIndex(stackIndices[s])

		sample.Values().Append(int64(w.rand.IntN(1000000)))

		if linkIndex > 0 {
			sample.SetLinkIndex(linkIndex)
		}
	}

	return nil
}

func (w *worker) addToBuffer(res pcommon.Map, telemetryAttributes []attribute.KeyValue, exporter profileExporter) error {
	w.bufferMutex.Lock()
	defer w.bufferMutex.Unlock()

	if w.batchBuffer == nil {
		td := pprofile.NewProfiles()
		w.batchBuffer = &td
	}

	if err := w.generateProfile(*w.batchBuffer, res, telemetryAttributes); err != nil {
		return err
	}
	w.bufferCount++

	if w.bufferCount >= w.batchSize {
		w.flushBuffer(exporter)
	}

	return nil
}

func (w *worker) flushBuffer(exporter profileExporter) {
	if w.batchBuffer == nil || w.bufferCount == 0 {
		return
	}

	if err := exporter.Export(context.Background(), *w.batchBuffer); err != nil {
		if w.allowFailures {
			w.logger.Error("failed to export batched profiles, continuing due to --allow-export-failures", zap.Error(err))
		} else {
			w.logger.Fatal("failed to export batched profiles", zap.Error(err))
		}
	} else {
		w.logger.Debug("exported batched profiles", zap.Int("count", w.bufferCount))
	}

	td := pprofile.NewProfiles()
	w.batchBuffer = &td
	w.bufferCount = 0
}
