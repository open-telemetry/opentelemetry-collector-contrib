package scrapertest

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/collector/model/otlp"
	"go.opentelemetry.io/collector/model/pdata"
)

func baseTestMetrics() pdata.MetricSlice {
	slice := pdata.NewMetricSlice()
	metric := slice.AppendEmpty()
	metric.SetDataType(pdata.MetricDataTypeGauge)
	metric.SetDescription("test description")
	metric.SetName("test name")
	metric.SetUnit("test unit")
	dps := metric.Gauge().DataPoints()
	dp := dps.AppendEmpty()
	dp.SetDoubleVal(1)
	dp.SetTimestamp(pdata.NewTimestampFromTime(time.Time{}))
	attributes := pdata.NewAttributeMap()
	attributes.Insert("testKey1", pdata.NewAttributeValueString("teststringvalue1"))
	attributes.CopyTo(dp.Attributes())
	return slice
}

// TestCompareMetrics tests the ability of comparing one metric slice to another
func TestCompareMetrics(t *testing.T) {
	tcs := []struct {
		name          string
		expected      pdata.MetricSlice
		actual        pdata.MetricSlice
		expectedError error
	}{
		{
			name:          "Equal MetricSlice",
			actual:        baseTestMetrics(),
			expected:      baseTestMetrics(),
			expectedError: nil,
		},
		{
			name: "Wrong DataType",
			actual: func() pdata.MetricSlice {
				metrics := baseTestMetrics()
				m := metrics.At(0)
				m.SetDataType(pdata.MetricDataTypeSum)
				return metrics
			}(),
			expected:      baseTestMetrics(),
			expectedError: fmt.Errorf("metric datatype does not match expected: Gauge, actual: Sum"),
		},
		{
			name: "Wrong Name",
			actual: func() pdata.MetricSlice {
				metrics := baseTestMetrics()
				m := metrics.At(0)
				m.SetName("wrong name")
				return metrics
			}(),
			expected:      baseTestMetrics(),
			expectedError: fmt.Errorf("metric name does not match expected: test name, actual: wrong name"),
		},
		{
			name: "Wrong Description",
			actual: func() pdata.MetricSlice {
				metrics := baseTestMetrics()
				m := metrics.At(0)
				m.SetDescription("wrong description")
				return metrics
			}(),
			expected:      baseTestMetrics(),
			expectedError: fmt.Errorf("metric description does not match expected: test description, actual: wrong description"),
		},
		{
			name: "Wrong Unit",
			actual: func() pdata.MetricSlice {
				metrics := baseTestMetrics()
				m := metrics.At(0)
				m.SetUnit("Wrong Unit")
				return metrics
			}(),
			expected:      baseTestMetrics(),
			expectedError: fmt.Errorf("metric Unit does not match expected: test unit, actual: Wrong Unit"),
		},
		{
			name: "Wrong doubleVal",
			actual: func() pdata.MetricSlice {
				metrics := baseTestMetrics()
				m := metrics.At(0)
				dp := m.Gauge().DataPoints().At(0)
				dp.SetDoubleVal(2)
				return metrics
			}(),
			expected:      baseTestMetrics(),
			expectedError: fmt.Errorf("metric datapoint DoubleVal doesn't match expected: 1.000000, actual: 2.000000"),
		},
		{
			name: "Wrong datatype",
			actual: func() pdata.MetricSlice {
				metrics := baseTestMetrics()
				m := metrics.At(0)
				dp := m.Gauge().DataPoints().At(0)
				dp.SetDoubleVal(0)
				dp.SetIntVal(2)
				return metrics
			}(),
			expected:      baseTestMetrics(),
			expectedError: fmt.Errorf("metric datapoint types don't match: expected type: 2, actual type: 1"),
		},
		{
			name: "Wrong attribute key",
			actual: func() pdata.MetricSlice {
				metrics := baseTestMetrics()
				m := metrics.At(0)
				dp := m.Gauge().DataPoints().At(0)
				attributes := pdata.NewAttributeMap()
				attributes.Insert("wrong key", pdata.NewAttributeValueString("teststringvalue1"))
				dp.Attributes().Clear()
				attributes.CopyTo(dp.Attributes())
				return metrics
			}(),
			expected:      baseTestMetrics(),
			expectedError: fmt.Errorf("missing datapoints for: metric name: test name"),
		},
		{
			name: "Wrong attribute value",
			actual: func() pdata.MetricSlice {
				metrics := baseTestMetrics()
				m := metrics.At(0)
				dp := m.Gauge().DataPoints().At(0)
				attributes := pdata.NewAttributeMap()
				attributes.Insert("testKey1", pdata.NewAttributeValueString("wrong value"))
				dp.Attributes().Clear()
				attributes.CopyTo(dp.Attributes())
				return metrics
			}(),
			expected:      baseTestMetrics(),
			expectedError: fmt.Errorf("missing datapoints for: metric name: test name"),
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := CompareMetrics(tc.expected, tc.actual)
			if tc.expectedError != nil {
				require.Equal(t, tc.expectedError, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestScraperTest ensures that ValidateScraper validates correctly
func TestValidateScraper(t *testing.T) {
	scrape := func(c context.Context) (pdata.ResourceMetricsSlice, error) {
		rms := pdata.NewResourceMetricsSlice()
		baseMetrics := baseTestMetrics()
		baseMetrics.CopyTo(rms.AppendEmpty().InstrumentationLibraryMetrics().AppendEmpty().Metrics())
		return rms, nil
	}

	newMetrics := pdata.NewMetrics()
	metricslice := newMetrics.ResourceMetrics()
	rms, _ := scrape(context.Background())
	rms.CopyTo(metricslice)

	expectedBytes, err := otlp.NewJSONMetricsMarshaler().MarshalMetrics(newMetrics)
	require.NoError(t, err)
	expectedFilePath := filepath.Join("testdata", "scrapers", "expected.json")
	err = ioutil.WriteFile(expectedFilePath, expectedBytes, 0600)
	require.NoError(t, err)
	ValidateScraper(t, scrape, expectedFilePath)
}
