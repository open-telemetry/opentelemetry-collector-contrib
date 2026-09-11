// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/internal/metadata"
)

// StandardFuncs is a helper function to provide quick access to all functions (editors and converters) in this package.
// Lambda functions are included only when the ottl.functions.enableLambda feature gate is enabled, and experimental
// functions only when the ottl.functions.enableExperimental feature gate is enabled; see the Function stability section
// of README.md.
func StandardFuncs[K any]() map[string]ottl.Factory[K] {
	f := []ottl.Factory[K]{
		// Editors
		NewClearFactory[K](),
		NewDeleteKeyFactory[K](),
		NewDeleteMatchingKeysFactory[K](),
		NewKeepMatchingKeysFactory[K](),
		NewFlattenFactory[K](),
		NewKeepKeysFactory[K](),
		NewLimitFactory[K](),
		NewMergeMapsFactory[K](),
		NewReplaceAllMatchesFactory[K](),
		NewReplaceAllPatternsFactory[K](),
		NewReplaceMatchFactory[K](),
		NewReplacePatternFactory[K](),
		NewSetFactory[K](),
		NewStringifyAllFactory[K](),
		NewTruncateAllFactory[K](),
	}
	f = append(f, converters[K]()...)
	f = append(f, lambdaFuncs[K]()...)
	f = append(f, experimentalFuncs[K]()...)

	return ottl.CreateFactoryMap(f...)
}

// StandardConverters is a helper function to provide quick access to all converters in this package.
// Lambda converters are included only when the ottl.functions.enableLambda feature gate is enabled, and experimental
// converters only when the ottl.functions.enableExperimental feature gate is enabled; see the Function stability section
// of README.md.
func StandardConverters[K any]() map[string]ottl.Factory[K] {
	c := converters[K]()
	c = append(c, lambdaFuncs[K]()...)
	c = append(c, experimentalFuncs[K]()...)
	return ottl.CreateFactoryMap(c...)
}

// lambdaFuncs returns the functions that require a lambda expression argument. Lambda expressions are an alpha language
// feature, so these functions are registered only when the ottl.functions.enableLambda feature gate is enabled.
func lambdaFuncs[K any]() []ottl.Factory[K] {
	if !metadata.OttlFunctionsEnableLambdaFeatureGate.IsEnabled() {
		return nil
	}
	return []ottl.Factory[K]{
		NewAllFactory[K](),
		NewAnyFactory[K](),
		NewFilterFactory[K](),
		NewFindFactory[K](),
		NewMapEachFactory[K](),
		NewMapKeysFactory[K](),
		NewReduceFactory[K](),
		NewWhenFactory[K](),
	}
}

func experimentalFuncs[K any]() []ottl.Factory[K] {
	if !metadata.OttlFunctionsEnableExperimentalFeatureGate.IsEnabled() {
		return nil
	}
	return []ottl.Factory[K]{}
}

func converters[K any]() []ottl.Factory[K] {
	return []ottl.Factory[K]{
		// Converters
		NewBase64EncodeFactory[K](),
		NewBoolFactory[K](),
		NewDecodeFactory[K](),
		NewCoalesceFactory[K](),
		NewCommunityIDFactory[K](),
		NewConcatFactory[K](),
		NewContainsValueFactory[K](),
		NewConvertCaseFactory[K](),
		NewConvertAttributesToElementsXMLFactory[K](),
		NewConvertTextToElementsXMLFactory[K](),
		NewDayFactory[K](),
		NewDoubleFactory[K](),
		NewDurationFactory[K](),
		NewExtractPatternsFactory[K](),
		NewExtractGrokPatternsFactory[K](),
		NewFnvFactory[K](),
		NewGetXMLFactory[K](),
		NewHasPrefixFactory[K](),
		NewHasSuffixFactory[K](),
		NewHourFactory[K](),
		NewHoursFactory[K](),
		NewIndexFactory[K](),
		NewInsertXMLFactory[K](),
		NewIntFactory[K](),
		NewIsBoolFactory[K](),
		NewIsDoubleFactory[K](),
		NewIsEmptyFactory[K](),
		NewIsListFactory[K](),
		NewIsIntFactory[K](),
		NewIsMapFactory[K](),
		NewIsMatchFactory[K](),
		NewIsStringFactory[K](),
		NewLenFactory[K](),
		NewLogFactory[K](),
		NewIsValidLuhnFactory[K](),
		NewMD5Factory[K](),
		NewMicrosecondsFactory[K](),
		NewMillisecondsFactory[K](),
		NewMinuteFactory[K](),
		NewMinutesFactory[K](),
		NewMonthFactory[K](),
		NewMurmur3HashFactory[K](),
		NewMurmur3Hash128Factory[K](),
		NewNanosecondFactory[K](),
		NewNanosecondsFactory[K](),
		NewNowFactory[K](),
		NewParseCSVFactory[K](),
		NewParseJSONFactory[K](),
		NewParseKeyValueFactory[K](),
		NewParseSimplifiedXMLFactory[K](),
		NewParseXMLFactory[K](),
		NewRemoveXMLFactory[K](),
		NewSecondFactory[K](),
		NewSecondsFactory[K](),
		NewSHA1Factory[K](),
		NewSHA256Factory[K](),
		NewSHA512Factory[K](),
		NewSortFactory[K](),
		NewSpanIDFactory[K](),
		NewSplitFactory[K](),
		NewFormatFactory[K](),
		NewStringFactory[K](),
		NewSubstringFactory[K](),
		NewTimeFactory[K](),
		NewFormatTimeFactory[K](),
		NewTrimFactory[K](),
		NewTrimPrefixFactory[K](),
		NewTrimSuffixFactory[K](),
		NewToKeyValueStringFactory[K](),
		NewToCamelCaseFactory[K](),
		NewToLowerCaseFactory[K](),
		NewToSnakeCaseFactory[K](),
		NewToUpperCaseFactory[K](),
		NewTruncateTimeFactory[K](),
		NewTraceIDFactory[K](),
		NewUnixFactory[K](),
		NewUnixMicroFactory[K](),
		NewUnixMilliFactory[K](),
		NewUnixNanoFactory[K](),
		NewUnixSecondsFactory[K](),
		NewUUIDFactory[K](),
		NewUUIDv7Factory[K](),
		NewURLFactory[K](),
		NewValuesFactory[K](),
		NewWeekdayFactory[K](),
		NewUserAgentFactory[K](),
		NewAppendFactory[K](),
		NewDeleteIndexFactory[K](),
		NewYearFactory[K](),
		NewHexFactory[K](),
		NewSliceToMapFactory[K](),
		NewParseSeverityFactory[K](),
		NewProfileIDFactory[K](),
		NewParseIntFactory[K](),
		NewKeysFactory[K](),
		NewXXH3Factory[K](),
		NewXXH128Factory[K](),
		NewIsInCIDRFactory[K](),
	}
}
