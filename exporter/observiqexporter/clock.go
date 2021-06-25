// Copyright  OpenTelemetry Authors
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

package observiqexporter

import "time"

// clock interface provides method for getting time. Allows mocking of time.
type clock interface {
	Now() time.Time
	AfterFunc(d time.Duration, f func()) *time.Timer
}

/*
	system clock uses time.Now() for getting time, and time.AfterFunc for AfterFunc.
*/
type systemClock struct{}

func (systemClock) Now() time.Time {
	return time.Now()
}

func (systemClock) AfterFunc(d time.Duration, f func()) *time.Timer {
	return time.AfterFunc(d, f)
}

var defaultClock = systemClock{}
