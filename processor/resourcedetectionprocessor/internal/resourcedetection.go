// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package internal contains an interface for detecting resource information,
// and a function to merge the resources returned by a slice of custom detectors.
//
// It also includes a default implementation to detect resource information
// from the OT_RESOURCE environment variable.
package internal

import (
	"context"

	"go.opentelemetry.io/collector/consumer/pdata"
)

type Detector interface {
	Detect(ctx context.Context) (pdata.Resource, error)
}

func Detect(ctx context.Context, detectors ...Detector) (pdata.Resource, error) {
	res := pdata.NewResource()
	res.InitEmpty()

	for _, detector := range detectors {
		r, err := detector.Detect(ctx)
		if err != nil {
			return pdata.NewResource(), err
		}
		MergeResource(res, r, false)
	}
	return res, nil
}

func MergeResource(to, from pdata.Resource, overrideTo bool) {
	if IsEmptyResource(from) {
		return
	}

	toAttr := to.Attributes()
	from.Attributes().ForEach(func(k string, v pdata.AttributeValue) {
		if overrideTo {
			toAttr.Upsert(k, v)
		} else {
			toAttr.Insert(k, v)
		}
	})
}

func IsEmptyResource(res pdata.Resource) bool {
	return res.Attributes().Len() == 0
}
