// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/promtailreceiver/internal"
import "github.com/prometheus/client_golang/prometheus"

// Unregisterer is a Prometheus Registerer that can unregister all collectors
// passed to it.
type Unregisterer struct {
	wrap prometheus.Registerer
	cs   map[prometheus.Collector]struct{}
}

func WrapWithUnregisterer(reg prometheus.Registerer) *Unregisterer {
	return &Unregisterer{
		wrap: reg,
		cs:   make(map[prometheus.Collector]struct{}),
	}
}

// Register implements prometheus.Registerer.
func (u *Unregisterer) Register(c prometheus.Collector) error {
	if u.wrap == nil {
		return nil
	}

	err := u.wrap.Register(c)
	if err != nil {
		return err
	}
	u.cs[c] = struct{}{}
	return nil
}

// MustRegister implements prometheus.Registerer.
func (u *Unregisterer) MustRegister(cs ...prometheus.Collector) {
	for _, c := range cs {
		if err := u.Register(c); err != nil {
			panic(err)
		}
	}
}

// Unregister implements prometheus.Registerer.
func (u *Unregisterer) Unregister(c prometheus.Collector) bool {
	if u.wrap != nil && u.wrap.Unregister(c) {
		delete(u.cs, c)
		return true
	}
	return false
}

// UnregisterAll unregisters all collectors that were registered through the Reigsterer.
func (u *Unregisterer) UnregisterAll() bool {
	success := true
	for c := range u.cs {
		if !u.Unregister(c) {
			success = false
		}
	}
	return success
}
