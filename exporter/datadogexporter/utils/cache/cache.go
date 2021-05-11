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

package cache

import (
	"time"

	gocache "github.com/patrickmn/go-cache"
)

const (
	CanonicalHostnameKey = "canonical_hostname"
	NoExpiration         = gocache.NoExpiration
	DefaultExpiration    = gocache.DefaultExpiration
)

var Cache = gocache.New(20*time.Minute, 10*time.Minute)
