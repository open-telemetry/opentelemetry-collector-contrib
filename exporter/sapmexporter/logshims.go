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

package sapmexporter

import (
	"github.com/signalfx/signalfx-agent/pkg/apm/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type zapShim struct {
	log *zap.Logger
}

func newZapShim(log *zap.Logger) zapShim {
	// Add caller skip so that the shim isn't logged as the caller.
	return zapShim{log.WithOptions(zap.AddCallerSkip(1))}
}

func (z zapShim) Debug(msg string) {
	z.log.Debug(msg)
}

func (z zapShim) Warn(msg string) {
	z.log.Warn(msg)
}

func (z zapShim) Error(msg string) {
	z.log.Error(msg)
}

func (z zapShim) Info(msg string) {
	z.log.Info(msg)
}

func (z zapShim) Panic(msg string) {
	z.log.Panic(msg)
}

func (z zapShim) WithFields(fields log.Fields) log.Logger {
	f := make([]zapcore.Field, 0, len(fields))
	for k, v := range fields {
		f = append(f, zap.Any(k, v))
	}
	return zapShim{z.log.With(f...)}
}

func (z zapShim) WithError(err error) log.Logger {
	return zapShim{z.log.With(zap.Error(err))}
}

var _ log.Logger = (*zapShim)(nil)
