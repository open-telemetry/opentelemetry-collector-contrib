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

package httpdreceiver

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/model/otlp"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

func TestScraper(t *testing.T) {
	httpdMock := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.String() == "/server-status?auto" {
			rw.WriteHeader(200)
			_, err := rw.Write([]byte(`ServerUptimeSeconds: 410
Total Accesses: 14169
Total kBytes: 20910
BusyWorkers: 13
IdleWorkers: 227
ConnsTotal: 110
Scoreboard: S_DD_L_GGG_____W__IIII_C________________W__________________________________.........................____WR______W____W________________________C______________________________________W_W____W______________R_________R________C_________WK_W________K_____W__C__________W___R______.............................................................................................................................
`))
			require.NoError(t, err)
			return
		}
		rw.WriteHeader(404)
	}))
	cfg := &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: fmt.Sprintf("%s%s", httpdMock.URL, "/server-status?auto"),
		},
	}
	require.NoError(t, cfg.Validate())
	sc := newHttpdScraper(zap.NewNop(), cfg)

	err := sc.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	assert.NotNil(t, sc.httpClient)

	actualMetrics := pdata.NewMetrics()
	rms := actualMetrics.ResourceMetrics()
	scrapedRMS, err := sc.scrape(context.Background())
	require.NoError(t, err)
	scrapedRMS.CopyTo(rms)

	expectedFile := filepath.Join("testdata", "scraper", "expected.json")
	expectedFileBytes, err := ioutil.ReadFile(expectedFile)
	require.NoError(t, err)
	unmarshaller := otlp.NewJSONMetricsUnmarshaler()
	expectedMetrics, err := unmarshaller.UnmarshalMetrics(expectedFileBytes)
	require.NoError(t, err)

	eMetricSlice := expectedMetrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics()
	aMetricSlice := actualMetrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics()

	require.NoError(t, compareMetrics(eMetricSlice, aMetricSlice))
}

func compareMetrics(expectedAll, actualAll pdata.MetricSlice) error {
	if actualAll.Len() != expectedAll.Len() {
		return fmt.Errorf("metrics not of same length")
	}

	lessFunc := func(a, b pdata.Metric) bool {
		return a.Name() < b.Name()
	}

	actualMetrics := actualAll.Sort(lessFunc)
	expectedMetrics := expectedAll.Sort(lessFunc)

	for i := 0; i < actualMetrics.Len(); i++ {
		actual := actualMetrics.At(i)
		expected := expectedMetrics.At(i)

		if actual.Name() != expected.Name() {
			return fmt.Errorf("metric name does not match expected: %s, actual: %s", expected.Name(), actual.Name())
		}
		if actual.DataType() != expected.DataType() {
			return fmt.Errorf("metric datatype does not match expected: %s, actual: %s", expected.DataType(), actual.DataType())
		}
		if actual.Description() != expected.Description() {
			return fmt.Errorf("metric description does not match expected: %s, actual: %s", expected.Description(), actual.Description())
		}
		if actual.Unit() != expected.Unit() {
			return fmt.Errorf("metric Unit does not match expected: %s, actual: %s", expected.Unit(), actual.Unit())
		}

		var actualDataPoints pdata.NumberDataPointSlice
		var expectedDataPoints pdata.NumberDataPointSlice

		switch actual.DataType() {
		case pdata.MetricDataTypeGauge:
			actualDataPoints = actual.Gauge().DataPoints()
			expectedDataPoints = expected.Gauge().DataPoints()
		case pdata.MetricDataTypeSum:
			if actual.Sum().AggregationTemporality() != expected.Sum().AggregationTemporality() {
				return fmt.Errorf("metric AggregationTemporality does not match expected: %s, actual: %s", expected.Sum().AggregationTemporality(), actual.Sum().AggregationTemporality())
			}
			if actual.Sum().IsMonotonic() != expected.Sum().IsMonotonic() {
				return fmt.Errorf("metric IsMonotonic does not match expected: %t, actual: %t", expected.Sum().IsMonotonic(), actual.Sum().IsMonotonic())
			}
			actualDataPoints = actual.Sum().DataPoints()
			expectedDataPoints = expected.Sum().DataPoints()
		}

		if actualDataPoints.Len() != expectedDataPoints.Len() {
			return fmt.Errorf("length of datapoints don't match")
		}

		dataPointMatches := 0
		for j := 0; j < expectedDataPoints.Len(); j++ {
			edp := expectedDataPoints.At(j)
			for k := 0; k < actualDataPoints.Len(); k++ {
				adp := actualDataPoints.At(k)
				adpAttributes := adp.Attributes()
				labelMatches := true

				if edp.Attributes().Len() != adpAttributes.Len() {
					break
				}
				edp.Attributes().Range(func(k string, v pdata.AttributeValue) bool {
					if attributeVal, ok := adpAttributes.Get(k); ok && attributeVal.StringVal() == v.StringVal() {
						return true
					}
					labelMatches = false
					return false
				})
				if !labelMatches {
					continue
				}
				if edp.IntVal() != adp.IntVal() {
					return fmt.Errorf("metric datapoint IntVal doesn't match expected: %d, actual: %d", edp.IntVal(), adp.IntVal())
				}
				if edp.DoubleVal() != adp.DoubleVal() {
					return fmt.Errorf("metric datapoint DoubleVal doesn't match expected: %f, actual: %f", edp.DoubleVal(), adp.DoubleVal())
				}
				dataPointMatches++
				break
			}
		}
		if dataPointMatches != expectedDataPoints.Len() {
			return fmt.Errorf("missing Datapoints")
		}
	}
	return nil
}

func TestScraperFailedStart(t *testing.T) {
	sc := newHttpdScraper(zap.NewNop(), &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "localhost:8080",
			TLSSetting: configtls.TLSClientSetting{
				TLSSetting: configtls.TLSSetting{
					CAFile: "/non/existent",
				},
			},
		},
	})
	err := sc.start(context.Background(), componenttest.NewNopHost())
	require.Error(t, err)
}

func TestParseScoreboard(t *testing.T) {

	t.Run("test freq count", func(t *testing.T) {
		scoreboard := `S_DD_L_GGG_____W__IIII_C________________W__________________________________.........................____WR______W____W________________________C______________________________________W_W____W______________R_________R________C_________WK_W________K_____W__C__________W___R______.............................................................................................................................`
		results := parseScoreboard(scoreboard)

		require.EqualValues(t, int64(150), results["open"])
		require.EqualValues(t, int64(217), results["waiting"])
		require.EqualValues(t, int64(1), results["starting"])
		require.EqualValues(t, int64(4), results["reading"])
		require.EqualValues(t, int64(12), results["sending"])
		require.EqualValues(t, int64(2), results["keepalive"])
		require.EqualValues(t, int64(2), results["dnslookup"])
		require.EqualValues(t, int64(4), results["closing"])
		require.EqualValues(t, int64(1), results["logging"])
		require.EqualValues(t, int64(3), results["finishing"])
		require.EqualValues(t, int64(4), results["idle_cleanup"])
	})

	t.Run("test unknown", func(t *testing.T) {
		scoreboard := `qwertyuiopasdfghjklzxcvbnm`
		results := parseScoreboard(scoreboard)

		require.EqualValues(t, int64(0), results["open"])
		require.EqualValues(t, int64(0), results["waiting"])
		require.EqualValues(t, int64(0), results["starting"])
		require.EqualValues(t, int64(0), results["reading"])
		require.EqualValues(t, int64(0), results["sending"])
		require.EqualValues(t, int64(0), results["keepalive"])
		require.EqualValues(t, int64(0), results["dnslookup"])
		require.EqualValues(t, int64(0), results["closing"])
		require.EqualValues(t, int64(0), results["logging"])
		require.EqualValues(t, int64(0), results["finishing"])
		require.EqualValues(t, int64(0), results["idle_cleanup"])
		require.EqualValues(t, int64(26), results["unknown"])
	})

	t.Run("test empty defaults", func(t *testing.T) {
		emptyString := ""
		results := parseScoreboard(emptyString)

		require.EqualValues(t, int64(0), results["open"])
		require.EqualValues(t, int64(0), results["waiting"])
		require.EqualValues(t, int64(0), results["starting"])
		require.EqualValues(t, int64(0), results["reading"])
		require.EqualValues(t, int64(0), results["sending"])
		require.EqualValues(t, int64(0), results["keepalive"])
		require.EqualValues(t, int64(0), results["dnslookup"])
		require.EqualValues(t, int64(0), results["closing"])
		require.EqualValues(t, int64(0), results["logging"])
		require.EqualValues(t, int64(0), results["finishing"])
		require.EqualValues(t, int64(0), results["idle_cleanup"])
	})
}

func TestParseStats(t *testing.T) {
	t.Run("with empty value", func(t *testing.T) {
		emptyString := ""
		require.EqualValues(t, map[string]string{}, parseStats(emptyString))
	})
	t.Run("with multi colons", func(t *testing.T) {
		got := "CurrentTime: Thursday, 17-Jun-2021 14:06:32 UTC"
		want := map[string]string{
			"CurrentTime": "Thursday, 17-Jun-2021 14:06:32 UTC",
		}
		require.EqualValues(t, want, parseStats(got))
	})
	t.Run("with header/footer", func(t *testing.T) {
		got := `localhost
ReqPerSec: 719.771
IdleWorkers: 227
ConnsTotal: 110
		`
		want := map[string]string{
			"ReqPerSec":   "719.771",
			"IdleWorkers": "227",
			"ConnsTotal":  "110",
		}
		require.EqualValues(t, want, parseStats(got))
	})
}

func TestScraperError(t *testing.T) {
	t.Run("no client", func(t *testing.T) {
		sc := newHttpdScraper(zap.NewNop(), &Config{})
		sc.httpClient = nil

		_, err := sc.scrape(context.Background())
		require.Error(t, err)
		require.EqualValues(t, errors.New("failed to connect to Apache HTTPd"), err)
	})
}
