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

package loadbalancingexporter

import "context"

type mockResolver struct {
	onStart           func(context.Context) error
	onShutdown        func(context.Context) error
	onResolve         func(ctx context.Context) ([]string, error)
	onChangeCallbacks []func([]string)
	triggerCallbacks  bool
}

func (m *mockResolver) start(ctx context.Context) error {
	if m.onStart != nil {
		if err := m.onStart(ctx); err != nil {
			return err
		}
	}

	_, err := m.resolve(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (m *mockResolver) shutdown(ctx context.Context) error {
	if m.onShutdown != nil {
		return m.onShutdown(ctx)
	}
	return nil
}

func (m *mockResolver) resolve(ctx context.Context) ([]string, error) {
	var resolved []string
	var err error
	if m.onResolve != nil {
		resolved, err = m.onResolve(ctx)
	}

	if m.triggerCallbacks {
		for _, callback := range m.onChangeCallbacks {
			callback(resolved)
		}
	}
	return resolved, err
}

func (m *mockResolver) onChange(f func([]string)) {
	m.onChangeCallbacks = append(m.onChangeCallbacks, f)
}

var _ resolver = (*mockResolver)(nil)
