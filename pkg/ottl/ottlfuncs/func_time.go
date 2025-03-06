// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/timeutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type TimeArguments[K any] struct {
	Time     ottl.StringGetter[K]
	Format   string
	Location ottl.Optional[string]
	Locale   ottl.Optional[string]
}

func NewTimeFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("Time", &TimeArguments[K]{}, createTimeFunction[K])
}

func createTimeFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*TimeArguments[K])

	if !ok {
		return nil, fmt.Errorf("TimeFactory args must be of type *TimeArguments[K]")
	}

	return Time(args.Time, args.Format, args.Location, args.Locale)
}

func Time[K any](inputTime ottl.StringGetter[K], format string, location ottl.Optional[string], locale ottl.Optional[string]) (ottl.ExprFunc[K], error) {
	if format == "" {
		return nil, fmt.Errorf("format cannot be nil")
	}
	gotimeFormat, err := timeutils.StrptimeToGotime(format)
	if err != nil {
		return nil, err
	}

	var defaultLocation *string
	if !location.IsEmpty() {
		l := location.Get()
		defaultLocation = &l
	}

	loc, err := timeutils.GetLocation(defaultLocation, &format)
	if err != nil {
		return nil, err
	}

	var inputTimeLocale *string
	if !locale.IsEmpty() {
		l := locale.Get()
		if err = timeutils.ValidateLocale(l); err != nil {
			return nil, err
		}
		inputTimeLocale = &l
	}

	return func(ctx context.Context, tCtx K) (any, error) {
		t, err := inputTime.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		if t == "" {
			return nil, fmt.Errorf("time cannot be nil")
		}
		var timestamp time.Time
		if inputTimeLocale != nil {
			timestamp, err = timeutils.ParseLocalizedGotime(gotimeFormat, t, loc, *inputTimeLocale)
		} else {
			timestamp, err = timeutils.ParseGotime(gotimeFormat, t, loc)
		}
		if err != nil {
			var timeErr *time.ParseError
			if errors.As(err, &timeErr) {
				toCTimeError(timeErr, format)
				return nil, timeErr
			}
			return nil, err
		}
		return timestamp, nil
	}, nil
}

func toCTimeError(parseError *time.ParseError, format string) {
	// set the layout to the originally provided ctime format
	parseError.Layout = format
	layoutElem, err := getCtimeSymbol(parseError.LayoutElem, format)
	if err == nil {
		parseError.LayoutElem = layoutElem
	}
}

var nativeToCtimeSubstitutes = map[string][]string{
	"2006":                     {"%Y"},
	"06":                       {"%y"},
	"01":                       {"%m"},
	"_1":                       {"%o"},
	"1":                        {"%q"},
	"Jan":                      {"%b", "%h"},
	"January":                  {"%B"},
	"02":                       {"%d"},
	"_2":                       {"%e"},
	"2":                        {"%g"},
	"Mon":                      {"%a"},
	"Monday":                   {"%A"},
	"15":                       {"%H"},
	"3":                        {"%l"},
	"03":                       {"%I"},
	"PM":                       {"%p"},
	"pm":                       {"%P"},
	"04":                       {"%M"},
	"05":                       {"%S"},
	"999":                      {"%L"},
	"999999":                   {"%f"},
	"99999999":                 {"%s"},
	"MST":                      {"%Z"},
	"Z0700":                    {"%z"},
	"-070000":                  {"%w"},
	"-07":                      {"%i"},
	"-07:00":                   {"%j"},
	"-07:00:00":                {"%k"},
	"01/02/2006":               {"%D", "%x"},
	"2006-01-02":               {"%F"},
	"15:04:05":                 {"%T", "%X"},
	"03:04:05 pm":              {"%r"},
	"15:04":                    {"%R"},
	"\n":                       {"%n"},
	"\t":                       {"%t"},
	"%":                        {"%%"},
	"Mon Jan 02 15:04:05 2006": {"%c"},
}

// FromNative converts Go native layout (which is used by time.Time.Format() and time.Parse() functions)
// to ctime-like format string.
func getCtimeSymbol(directive, format string) (string, error) {
	if subst, ok := nativeToCtimeSubstitutes[directive]; ok {
		if len(subst) == 1 {
			return subst[1], nil
		}

		// some symbols can map to multiple ctime directives, such as "Jan" either maps to "%b" or %h"
		// therefore we need to check the original format for either of the possible symbols and use that one that has been used originally
		for _, sub := range subst {
			if strings.Contains(format, sub) {
				return sub, nil
			}
		}
	}
	return "", fmt.Errorf("unsupported time directive: %s", directive)
}
