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

package resourcedetectionprocessor

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

type lazyResource func() (pdata.Resource, error)

func getDetectors(ctx context.Context, allDetectors map[string]internal.Detector, detectorNames []string) ([]internal.Detector, error) {
	detectors := make([]internal.Detector, 0, len(detectorNames))
	for _, key := range detectorNames {
		detector, ok := allDetectors[key]
		if !ok {
			return nil, fmt.Errorf("invalid detector key: %s", key)
		}

		detectors = append(detectors, detector)
	}

	return detectors, nil
}

func getLazyResource(ctx context.Context, logger *zap.Logger, timeout time.Duration, detectors []internal.Detector) lazyResource {
	var once sync.Once
	var resource pdata.Resource
	return func() (res pdata.Resource, err error) {
		once.Do(func() { resource, err = detectResource(ctx, logger, timeout, detectors) })
		res = resource
		return
	}
}

type detectResourceResult struct {
	resource pdata.Resource
	err      error
}

func detectResource(ctx context.Context, logger *zap.Logger, timeout time.Duration, detectors []internal.Detector) (pdata.Resource, error) {
	var resource pdata.Resource
	ch := make(chan detectResourceResult, 1)

	logger.Info("started detecting resource information")
	go func() {
		res, err := internal.Detect(ctx, detectors...)
		ch <- detectResourceResult{resource: res, err: err}
	}()

	select {
	case <-time.After(timeout):
		return resource, errors.New("timeout attempting to detect resource information")
	case rst := <-ch:
		if rst.err != nil {
			return resource, rst.err
		}

		resource = rst.resource
	}

	logger.Info("completed detecting resource information", zap.Any("resource", resourceToMap(resource)))
	return resource, nil
}

func resourceToMap(resource pdata.Resource) map[string]interface{} {
	mp := make(map[string]interface{}, resource.Attributes().Len())

	resource.Attributes().ForEach(func(k string, v pdata.AttributeValue) {
		switch v.Type() {
		case pdata.AttributeValueBOOL:
			mp[k] = v.BoolVal()
		case pdata.AttributeValueINT:
			mp[k] = v.IntVal()
		case pdata.AttributeValueDOUBLE:
			mp[k] = v.DoubleVal()
		case pdata.AttributeValueSTRING:
			mp[k] = v.StringVal()
		}
	})

	return mp
}
