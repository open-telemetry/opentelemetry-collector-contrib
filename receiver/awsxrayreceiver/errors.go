// Copyright 2019, OpenTelemetry Authors
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

package awsxrayreceiver

type errRecoverable struct {
	err error
}

func (e *errRecoverable) Error() string {
	return e.err.Error()
}

func (e *errRecoverable) Unwrap() error {
	return e.err
}

type errIrrecoverable struct {
	err error
}

func (e *errIrrecoverable) Error() string {
	return e.err.Error()
}

func (e *errIrrecoverable) Unwrap() error {
	return e.err
}
