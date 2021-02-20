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

package logproto

import "github.com/prometheus/prometheus/pkg/labels"

// Note, this is not very efficient and use should be minimized as it requires label construction on each comparison
type SeriesIdentifiers []SeriesIdentifier

func (ids SeriesIdentifiers) Len() int      { return len(ids) }
func (ids SeriesIdentifiers) Swap(i, j int) { ids[i], ids[j] = ids[j], ids[i] }
func (ids SeriesIdentifiers) Less(i, j int) bool {
	a, b := labels.FromMap(ids[i].Labels), labels.FromMap(ids[j].Labels)
	return labels.Compare(a, b) <= 0
}

type Streams []Stream

func (xs Streams) Len() int           { return len(xs) }
func (xs Streams) Swap(i, j int)      { xs[i], xs[j] = xs[j], xs[i] }
func (xs Streams) Less(i, j int) bool { return xs[i].Labels <= xs[j].Labels }

func (s Series) Len() int           { return len(s.Samples) }
func (s Series) Swap(i, j int)      { s.Samples[i], s.Samples[j] = s.Samples[j], s.Samples[i] }
func (s Series) Less(i, j int) bool { return s.Samples[i].Timestamp < s.Samples[j].Timestamp }
