// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampler

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// SamplingParameters is part of the original OTel-Go Sampling API.
//
// We should be aware that there are standing requests to extend it
// with at least three more fields:
// - SpanID: controversial because the spec says it's created after ShouldSample()
// - Scope: controversial because it's a static property
// - Resource: controversial because it's a static property
type SamplingParameters struct {
	ParentContext context.Context
	TraceID       trace.TraceID
	Name          string
	Kind          trace.SpanKind
	Attributes    []attribute.KeyValue
	Links         []trace.Link
}

// Sampler is part of the original OTel-Go Sampling API.
//
// We refer to this API as non-compositional because it does not
// separate its intentions from its side-effects.  This prototype
// introduces "composable" forms of Sampler and SamplingParameters.
type Sampler interface {
	// ShouldSample is called prior to constructing a Span.
	ShouldSample(SamplingParameters) SamplingResult

	// Description is used when logging SDK configuration.
	Description() string
}

// SamplingDecision is part of the original OTel-Go Sampling API.
type SamplingDecision uint8

const (
	Drop SamplingDecision = iota
	RecordOnly
	ExportOnly // TODO: This is new. How can it be added in Go w/o breaking changes?
	RecordAndSample
)

// SamplingResult is part of the original OTel-Go Sampling API.
//
// In this prototype, we aim to lower the cost of composite sampler
// decisions by deferring the construction of attributes and tracestate
// where the decision is combined from multiple samplers.
type SamplingResult struct {
	Decision   SamplingDecision
	Attributes []attribute.KeyValue
	Tracestate trace.TraceState
}

// ComposableSamplingParameters extend SamplingParameters.
//
// Since this stands as a proposal to extend the OTel Sampling API, it
// seems worth examining other standing feature requests.  Users would
// like their Samplers to have access to Scope and Resource, which are
// static properties, and the SpanID which the specification says not to
// include.
type ComposableSamplingParameters struct {
	// SamplingParameters are the original API parameters.
	SamplingParameters

	// ParentSpanContext equals trace.SpanContextFromContext(p.ParentContext)
	//
	// This is an expensive call, so we compute it once in case
	// multiple predicates will use it.
	ParentSpanContext trace.SpanContext

	// parentThreshold is only for use by the ParentThreshold
	// sampler, thus not exported.  When there is no incoming
	// threshold and sampled, initialize to INVALID_THRESHOLD,
	// otherwise initialize to NEVER_SAMPLE_THRESHOLD when not
	// sampled.
	parentThreshold int64

	// parentThresholdReliable indicates whether the thresohld
	// was defined (reliable) or not, because a context had the
	// sampled flag and no threshold.
	parentThresholdReliable bool
}

// ComposableSampler is a sampler which separates its intentions from
// its side-effects.
type ComposableSampler interface {
	// GetSamplingIntent returns the threshold at which this Sampler
	// wishes to sample and functions that defer the side-effects of
	// a positive decision.
	GetSamplingIntent(ComposableSamplingParameters) SamplingIntent

	// Description is used when logging SDK configuration.
	Description() string
}

// AttributesFunc is a function that returns a set of attributes.
type AttributesFunc func() []attribute.KeyValue

// TraceStateFunc is a function that modifies a TraceState.
type TraceStateFunc func(trace.TraceState) trace.TraceState

// SamplingIntent returns this sampler's intention.
type SamplingIntent struct {
	Threshold         int64          // i.e., sampling probability, implies record & export when...
	ThresholdReliable bool           // whether the threshold is reliable
	Attributes        AttributesFunc // add attributes the span
	TraceState        TraceStateFunc // update the tracestate
}

// TraceIDRatioBased is the OTel-specified probabilistic sampler. This was
// defined in OTEP 235.
//
// Note: Add support for variable precision? This has been done in e.g.,
// https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/9b515fb83b3f010c4c37f3135caf535e391fb3a3/pkg/sampling/probability.go#L33
func TraceIDRatioBased(fraction float64) ComposableSampler {
	const (
		maxp  = 14                       // maximum precision is 56 bits
		defp  = defaultSamplingPrecision // default precision
		hbits = 4                        // bits per hex digit
	)

	if fraction > maxSupportedProbability {
		return ComposableAlwaysSample()
	}

	if fraction < minSupportedProbability {
		return ComposableNeverSample()
	}

	// Calculate the amount of precision needed to encode the
	// threshold with reasonable precision.
	//
	// 13 hex digits is the maximum reasonable precision, since
	// that equals 52 bits, the number of bits in the float64
	// significand.
	//
	// Frexp() normalizes both the fraction and one-minus the
	// fraction, because more digits of precision are needed in
	// both cases -- in these cases the threshold has all leading
	// '0' or 'f' characters.
	//
	// We know that `exp <= 0`.  If `exp <= -4`, there will be a
	// leading hex `0` or `f`.  For every multiple of -4, another
	// leading `0` or `f` appears, so this raises precision
	// accordingly.
	_, expF := math.Frexp(fraction)
	precision := min(maxp, defp+expF/-hbits)

	// Compute the threshold
	scaled := uint64(math.Round(fraction * float64(maxAdjustedCount)))
	threshold := maxAdjustedCount - scaled

	// Round to the specified precision, if less than the maximum.
	if shift := hbits * (maxp - precision); shift != 0 {
		half := uint64(1) << (shift - 1)
		threshold += half
		threshold >>= shift
		threshold <<= shift
	}

	return &traceIDRatio{
		threshold:   threshold,
		description: fmt.Sprintf("TraceIDRatioBased{%g}", fraction),
	}
}

type traceIDRatio struct {
	// threshold is a rejection threshold.
	// Select when (T <= R)
	// Drop when (T > R)
	// Range is [0, 1<<56).
	threshold   uint64
	description string
}

var _ ComposableSampler = &traceIDRatio{}

// Description implements ComposableSampler.
func (ts *traceIDRatio) Description() string {
	return ts.description
}

// GetSamplingIntent implements ComposableSampler.
func (ts *traceIDRatio) GetSamplingIntent(p ComposableSamplingParameters) SamplingIntent {
	return SamplingIntent{
		Threshold:         int64(ts.threshold),
		ThresholdReliable: true,
	}
}

// AlwaysOn is could be defined as:
//
//	CompositeSampler(ComposableAlwaysSample())
//
// and this is defined so we can measure the abstraction cost.
func AlwaysSample() Sampler {
	return alwaysOn{}
}

type alwaysOn struct{}

var _ Sampler = alwaysOn{}

func (alwaysOn) ShouldSample(params SamplingParameters) SamplingResult {
	return SamplingResult{
		Decision:   RecordAndSample,
		Tracestate: trace.SpanContextFromContext(params.ParentContext).TraceState(),
	}
}

// Description implements ComposableSampler.
func (alwaysOn) Description() string {
	return "AlwaysOn"
}

// ComposableAlwaysOn

func ComposableAlwaysSample() ComposableSampler {
	return cAlwaysOn{}
}

type cAlwaysOn struct{}

var _ ComposableSampler = cAlwaysOn{}

// GetSamplingIntent implements ComposableSampler.
func (cAlwaysOn) GetSamplingIntent(ComposableSamplingParameters) SamplingIntent {
	return SamplingIntent{
		Threshold:         ALWAYS_SAMPLE_THRESHOLD,
		ThresholdReliable: true,
	}
}

// Description implements ComposableSampler.
func (cAlwaysOn) Description() string {
	return "AlwaysOn"
}

// AlwaysOff

func NeverSample() Sampler {
	return CompositeSampler(ComposableNeverSample())
}

func ComposableNeverSample() ComposableSampler {
	return alwaysOff{}
}

type alwaysOff struct{}

var _ ComposableSampler = alwaysOff{}

// GetSamplingIntent implements ComposableSampler.
func (alwaysOff) GetSamplingIntent(ComposableSamplingParameters) SamplingIntent {
	return SamplingIntent{
		Threshold: NEVER_SAMPLE_THRESHOLD,
	}
}

// Description implements ComposableSampler.
func (alwaysOff) Description() string {
	return "AlwaysOff"
}

// RuleBased is a composite sampler that selects a delegate sampler based on a set of rules.
func RuleBased(options ...RuleBasedOption) ComposableSampler {
	rbc := &ruleBasedConfig{}
	for _, opt := range options {
		opt(rbc)
	}
	if rbc.defRule != nil {
		rbc.rules = append(rbc.rules, ruleAndPredicate{
			Predicate:         TruePredicate(),
			ComposableSampler: rbc.defRule,
		})
	}
	return ruleBased(rbc.rules)
}

type ruleAndPredicate struct {
	Predicate
	ComposableSampler
}

type ruleBasedConfig struct {
	rules   []ruleAndPredicate
	defRule ComposableSampler
}

type ruleBased []ruleAndPredicate

var _ ComposableSampler = &ruleBased{}

// Description implements ComposableSampler.
func (rb ruleBased) Description() string {
	return fmt.Sprintf("RuleBased{%s}",
		strings.Join(func(rules []ruleAndPredicate) (desc []string) {
			for _, rule := range rules {
				desc = append(desc,
					fmt.Sprintf("rule(%s)=%s",
						rule.Predicate.Description(),
						rule.ComposableSampler.Description(),
					),
				)
			}
			return
		}(rb), ","))
}

// GetSamplingIntent implements ComposableSampler.
func (rb ruleBased) GetSamplingIntent(params ComposableSamplingParameters) SamplingIntent {
	for _, rule := range rb {
		if rule.Decide(params) {
			return rule.ComposableSampler.GetSamplingIntent(params)
		}
	}

	// When no rules match.  This will not happen when there is a
	// default rule set.
	return SamplingIntent{
		Threshold: NEVER_SAMPLE_THRESHOLD,
	}
}

// ComposableParentBased combines a root sampler and a ParentThreshold.
func ComposableParentBased(root ComposableSampler) ComposableSampler {
	return RuleBased(
		WithRule(IsRootPredicate(), root),
		WithDefaultRule(ParentThreshold()),
	)
}

// ParentThreshold may be composed to form consistent parent-based sampling.
func ParentThreshold() ComposableSampler {
	return parentThreshold{}
}

type parentThreshold struct{}

var _ ComposableSampler = &parentThreshold{}

// GetSamplingIntent implements ComposableSampler.
func (parentThreshold) GetSamplingIntent(params ComposableSamplingParameters) SamplingIntent {
	return SamplingIntent{
		Threshold:         params.parentThreshold,
		ThresholdReliable: params.parentThresholdReliable,
	}
}

// Description implements ComposableSampler.
func (parentThreshold) Description() string {
	return "ParentThreshold"
}

// Annotating (a.k.a. "Marker")

type AnnotatingOption func(*annotatingConfig)

type annotatingConfig struct {
	attributes AttributesFunc
}

type annotatingSampler struct {
	sampler    ComposableSampler
	attributes AttributesFunc
}

var _ ComposableSampler = &annotatingSampler{}

func AnnotatingSampler(sampler ComposableSampler, options ...AnnotatingOption) ComposableSampler {
	var config annotatingConfig
	for _, opt := range options {
		opt(&config)
	}
	return &annotatingSampler{
		sampler:    sampler,
		attributes: config.attributes,
	}
}

func combineAttributesFunc(one, two AttributesFunc) AttributesFunc {
	return func() []attribute.KeyValue {
		if one == nil && two == nil {
			return nil
		}
		if one == nil {
			return two()
		}
		if two == nil {
			return one()
		}
		return append(one(), two()...)
	}
}

func WithSampledAttributes(af AttributesFunc) AnnotatingOption {
	return func(cfg *annotatingConfig) {
		cfg.attributes = combineAttributesFunc(cfg.attributes, af)
	}
}

// GetSamplingIntent implements ComposableSampler.
func (as annotatingSampler) GetSamplingIntent(params ComposableSamplingParameters) SamplingIntent {
	intent := as.sampler.GetSamplingIntent(params)
	intent.Attributes = combineAttributesFunc(intent.Attributes, as.attributes)
	return intent
}

// Description implements ComposableSampler.
func (as annotatingSampler) Description() string {
	set := attribute.NewSet(as.attributes()...)
	return fmt.Sprintf("Annotate(%s, %s)", as.sampler.Description(), attribute.DefaultEncoder().Encode(set.Iter()))
}

// CompositeSampler construct a Sampler from a ComposableSampler.
func CompositeSampler(s ComposableSampler) Sampler {
	return &compositeSampler{
		sampler: s,
	}
}

type compositeSampler struct {
	sampler ComposableSampler
}

var _ Sampler = &compositeSampler{}

// ShouldSample implements Sampler.
func (c *compositeSampler) ShouldSample(params SamplingParameters) SamplingResult {
	// Note: I experimented with making the steps below be lazy,
	// since not all Sampler configurations will use the results,
	// by using sync.Once and a func().  This isn't worthwhile
	// because the additional allocations counteract the savings:
	// - trace.SpanContextFromContext
	// - TraceState().Get("ot")
	// - tracestateHasThreshold()
	// - tracestateHasRandomness()
	// In benchmarking, it's substantially faster to just run
	// through these calls w/o allocations.

	psc := trace.SpanContextFromContext(params.ParentContext)
	returnTracestate := psc.TraceState()
	otts := returnTracestate.Get("ot")

	parsedThreshold, saveThresholdPos, hasThreshold := tracestateHasThreshold(otts)
	threshold := parsedThreshold

	var hasRandom bool
	var rnd int64
	if otts != "" {
		// When the OTel trace state field exists, we will
		// inspect for a "rv" and "th", otherwise assume that the
		// TraceID is random.
		rnd, hasRandom = tracestateHasRandomness(otts)
	}
	if !hasRandom {
		// Interpret the least-significant 8-bytes as an
		// unsigned number, then zero the top 8 bits using
		// randomnessMask, yielding the least-significant 56
		// bits of randomness, as specified in W3C Trace
		// Context Level 2.
		rnd = int64(binary.BigEndian.Uint64(params.TraceID[8:16]) & randomnessMask)
	}

	// thresholdReliable indicates whether the threshold is reliable
	// in terms defined in #4321.
	thresholdReliable := false
	switch {
	case hasThreshold:
		// Validate the threshold.
		tsampled := threshold <= rnd
		fsampled := psc.IsSampled()

		switch {
		case tsampled && fsampled:
			// Good. The two agree.
			thresholdReliable = true
		case tsampled:
			// Threshold says sampled, flag says not.
			psc = psc.WithTraceFlags(psc.TraceFlags() | trace.FlagsSampled)
			thresholdReliable = true
		case fsampled:
			// Flag says sampled, threshold says not. This erases the invalid threshold.
			threshold = INVALID_THRESHOLD
		default:
			// Good. The two agree.
		}

	case psc.IsSampled():
		threshold = INVALID_THRESHOLD
	default:
		threshold = NEVER_SAMPLE_THRESHOLD
	}

	intent := c.sampler.GetSamplingIntent(ComposableSamplingParameters{
		SamplingParameters:      params,
		ParentSpanContext:       psc,
		parentThreshold:         threshold,
		parentThresholdReliable: thresholdReliable,
	})

	var sampled bool
	switch {
	case intent.Threshold >= NEVER_SAMPLE_THRESHOLD:
		sampled = false
	case intent.Threshold <= ALWAYS_SAMPLE_THRESHOLD:
		sampled = true
	default:
		sampled = intent.Threshold <= rnd
	}

	var decision SamplingDecision
	var attrs []attribute.KeyValue
	var err error
	switch {
	case sampled:
		decision = RecordAndSample
		if intent.Attributes != nil {
			attrs = intent.Attributes()
		}
		returnTracestate, err = combineTracestate(returnTracestate, intent.Threshold, intent.ThresholdReliable, parsedThreshold, saveThresholdPos, hasThreshold)
	default:
		decision = Drop
	}
	if err != nil {
		otel.Handle(fmt.Errorf("tracestate: %w", err))
	}

	return SamplingResult{
		Attributes: attrs,
		Tracestate: returnTracestate,
		Decision:   decision,
	}
}

// Description implements ComposableSampler.
func (c *compositeSampler) Description() string {
	return c.sampler.Description()
}
