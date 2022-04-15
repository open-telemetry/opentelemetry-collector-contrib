// Copyright 2020, OpenTelemetry Authors
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

//go:build windows
// +build windows

package sqlserverreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
)

func TestSqlServerScraper(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	s := newSqlServerScraper(
		componenttest.NewNopReceiverCreateSettings().Logger,
		cfg,
	)
	s.start(context.Background(), nil)
	require.Equal(t, len(s.watchers), 21)
	data, err := s.scrape(context.Background())
	require.NoError(t, err)
	require.NotNil(t, data)
	err = s.shutdown(context.Background())
	require.NoError(t, err)
}
