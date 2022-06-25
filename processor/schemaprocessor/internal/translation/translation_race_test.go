// Copyright  The OpenTelemetry Authors
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

package translation

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/fixture"
)

func TestRaceTranslationMultiRead(t *testing.T) {
	t.Parallel()

	tn, err := newTranslater(zaptest.NewLogger(t), "https://opentelemetry.io/schemas/1.9.0")
	require.NoError(t, err, "Must not error when creating translater")
	require.NoError(t, tn.merge(LoadTranslationVersion(t, TranslationVersion190)), "Must not have issues trying to convert into translator")

	ctx, done := context.WithCancel(context.Background())
	t.Cleanup(done)

	fixture.ParallelRaceCompute(t, 10, func() error {
		for i := 0; i < 10; i++ {
			count := 0
			v := &Version{1, 0, 0}
			it, _ := tn.iterator(ctx, v)
			for rev, more := it(); more; rev, more = it() {
				switch count {
				case 0:
					if !assert.True(t, v.Equal(rev.Version())) {
						done()
						return errors.New("incorrect progression through chanages")
					}
				default:
					if !assert.True(t, v.LessThan(rev.Version()), "Must be increasing with each revision", v, rev.Version()) {
						done()
						return errors.New("incorrect progression through chanages")
					}
				}

				count++
				v = rev.Version()
			}
		}
		return nil
	})
}
