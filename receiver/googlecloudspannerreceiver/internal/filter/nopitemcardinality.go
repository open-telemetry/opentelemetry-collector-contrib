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

package filter // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/filter"

type nopItemCardinalityFilter struct {
	// No fields here
}

type nopItemFilterResolver struct {
	nopFilter *nopItemCardinalityFilter
}

func NewNopItemCardinalityFilter() ItemFilter {
	return &nopItemCardinalityFilter{}
}

func NewNopItemFilterResolver() ItemFilterResolver {
	return &nopItemFilterResolver{
		nopFilter: &nopItemCardinalityFilter{},
	}
}

func (f *nopItemCardinalityFilter) Filter(sourceItems []*Item) ([]*Item, error) {
	return sourceItems, nil
}

func (f *nopItemCardinalityFilter) Shutdown() error {
	return nil
}

func (f *nopItemCardinalityFilter) TotalLimit() int {
	return 0
}

func (f *nopItemCardinalityFilter) LimitByTimestamp() int {
	return 0
}

func (r *nopItemFilterResolver) Resolve(string) (ItemFilter, error) {
	return r.nopFilter, nil
}

func (r *nopItemFilterResolver) Shutdown() error {
	return nil
}
