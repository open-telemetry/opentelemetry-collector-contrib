// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cms

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCmsCountAfterInsertNoUnderEstimate(t *testing.T) {
	maxValue := 1000

	testCases := []struct {
		ErrorProbability float64
		ErrorBound       float64
	}{
		{
			ErrorProbability: .01,
			ErrorBound:       1.,
		},

		{
			ErrorProbability: .05,
			ErrorBound:       1.,
		},

		{
			ErrorProbability: .10,
			ErrorBound:       2.,
		},

		{
			ErrorProbability: .20,
			ErrorBound:       2.,
		},
	}

	for _, c := range testCases {
		caseName := fmt.Sprintf("error_prob_%.2f_error_bound_%.0f", c.ErrorProbability, c.ErrorBound)
		t.Run(caseName, func(t *testing.T) {
			cms := NewCMSWithErrorParams(&CountMinSketchCfg{
				ErrorProbability: c.ErrorProbability,
				TotalFreq:        float64(countOfInsertions(maxValue)),
				MaxErr:           c.ErrorBound,
			})

			for i := 1; i < maxValue+1; i++ {
				bytesData := []byte(strconv.Itoa(i))
				for k := 0; k < i; k++ {
					cms.Insert(bytesData)
				}
			}

			for i := 1; i < maxValue+1; i++ {
				bytesData := []byte(strconv.Itoa(i))
				cnt := int(cms.Count(bytesData))
				assert.GreaterOrEqual(t, cnt, i, "estimated cnt (%d) is less than actual (%d)", cnt, i)
			}
		})
	}
}

func TestCmsCountAfterInsertErrorBound(t *testing.T) {
	maxValue := 1000

	testCases := []struct {
		ErrorProbability float64
		ErrorBound       float64
	}{
		{
			ErrorProbability: .01,
			ErrorBound:       1.,
		},

		{
			ErrorProbability: .05,
			ErrorBound:       1.,
		},

		{
			ErrorProbability: .10,
			ErrorBound:       2.,
		},

		{
			ErrorProbability: .20,
			ErrorBound:       2.,
		},
	}

	for _, c := range testCases {
		caseName := fmt.Sprintf("error_prob_%.2f_error_bound_%.0f", c.ErrorProbability, c.ErrorBound)
		t.Run(caseName, func(t *testing.T) {
			cms := NewCMSWithErrorParams(&CountMinSketchCfg{
				ErrorProbability: c.ErrorProbability,
				TotalFreq:        float64(countOfInsertions(maxValue)),
				MaxErr:           c.ErrorBound,
			})

			for i := 1; i < maxValue+1; i++ {
				bytesData := makeTestCMSKey(i)
				for k := 0; k < i; k++ {
					cms.Insert(bytesData)
				}
			}

			for i := 1; i < maxValue+1; i++ {
				bytesData := makeTestCMSKey(i)
				cnt := int(cms.Count(bytesData))
				errValue := cnt - i
				assert.LessOrEqualf(t, errValue, int(c.ErrorBound),
					"error(%d) is greater than defined(%d); i = %d",
					errValue, int(c.ErrorBound), i)
			}
		})
	}
}

func TestCmsCountAfterInsertErrorProbability(t *testing.T) {
	maxValue := 1000

	testCases := []struct {
		ErrorProbability float64
		ErrorBound       float64
	}{
		{
			ErrorProbability: .01,
			ErrorBound:       1.,
		},

		{
			ErrorProbability: .05,
			ErrorBound:       1.,
		},

		{
			ErrorProbability: .10,
			ErrorBound:       2.,
		},

		{
			ErrorProbability: .20,
			ErrorBound:       2.,
		},
	}

	for _, c := range testCases {
		caseName := fmt.Sprintf("error_prob_%.2f_error_bound_%.0f", c.ErrorProbability, c.ErrorBound)
		t.Run(caseName, func(t *testing.T) {
			cms := NewCMSWithErrorParams(&CountMinSketchCfg{
				ErrorProbability: c.ErrorProbability,
				TotalFreq:        float64(countOfInsertions(maxValue)),
				MaxErr:           c.ErrorBound,
			})

			for i := 1; i < maxValue+1; i++ {
				bytesData := makeTestCMSKey(i)
				for k := 0; k < i; k++ {
					cms.Insert(bytesData)
				}
			}

			overestimated := .0
			for i := 1; i < maxValue+1; i++ {
				bytesData := makeTestCMSKey(i)
				cnt := int(cms.Count(bytesData))
				if cnt != i {
					overestimated++
				}
			}
			errProb := overestimated / float64(countOfInsertions(maxValue))
			assert.LessOrEqualf(t, errProb, c.ErrorProbability,
				"%s: error(%.4f) is greater than defined(%.2f)",
				caseName, errProb, c.ErrorProbability)
		})
	}
}

func TestCmsInsertWithCountNoUnderEstimate(t *testing.T) {
	maxValue := 1000

	testCases := []struct {
		ErrorProbability float64
		ErrorBound       float64
	}{
		{
			ErrorProbability: .01,
			ErrorBound:       1.,
		},

		{
			ErrorProbability: .05,
			ErrorBound:       1.,
		},

		{
			ErrorProbability: .10,
			ErrorBound:       2.,
		},

		{
			ErrorProbability: .20,
			ErrorBound:       2.,
		},
	}

	for _, c := range testCases {
		caseName := fmt.Sprintf("error_prob_%.2f_error_bound_%.0f", c.ErrorProbability, c.ErrorBound)
		t.Run(caseName, func(t *testing.T) {
			cms := NewCMSWithErrorParams(&CountMinSketchCfg{
				ErrorProbability: c.ErrorProbability,
				TotalFreq:        float64(countOfInsertions(maxValue)),
				MaxErr:           c.ErrorBound,
			})

			for i := 1; i < maxValue+1; i++ {
				bytesData := []byte(strconv.Itoa(i))
				for k := 0; k < i-1; k++ {
					cms.InsertWithCount(bytesData)
				}
				cnt := int(cms.InsertWithCount(bytesData))
				assert.GreaterOrEqualf(t, cnt, i, "estimated cnt (%d) is less than actual (%d)", cnt, i)
			}
		})
	}
}

func TestCmsInsertWithCountErrorBound(t *testing.T) {
	maxValue := 1000

	testCases := []struct {
		ErrorProbability float64
		ErrorBound       float64
	}{
		{
			ErrorProbability: .01,
			ErrorBound:       1.,
		},

		{
			ErrorProbability: .05,
			ErrorBound:       1.,
		},

		{
			ErrorProbability: .10,
			ErrorBound:       2.,
		},

		{
			ErrorProbability: .20,
			ErrorBound:       2.,
		},
	}

	for _, c := range testCases {
		caseName := fmt.Sprintf("error_prob_%.2f_error_bound_%.0f", c.ErrorProbability, c.ErrorBound)
		t.Run(caseName, func(t *testing.T) {
			cms := NewCMSWithErrorParams(&CountMinSketchCfg{
				ErrorProbability: c.ErrorProbability,
				TotalFreq:        float64(countOfInsertions(maxValue)),
				MaxErr:           c.ErrorBound,
			})

			for i := 1; i < maxValue+1; i++ {
				bytesData := makeTestCMSKey(i)
				for k := 0; k < i-1; k++ {
					cms.InsertWithCount(bytesData)
				}
				cnt := cms.InsertWithCount(bytesData)
				errValue := int(cnt) - i
				assert.LessOrEqualf(t, errValue, int(c.ErrorBound),
					"error(%d) is greater than defined(%d); i = %d",
					errValue, int(c.ErrorBound), i)
			}
		})
	}
}

func TestCmsInsertWithCountErrorProbability(t *testing.T) {
	maxValue := 1000

	testCases := []struct {
		ErrorProbability float64
		ErrorBound       float64
	}{
		{
			ErrorProbability: .01,
			ErrorBound:       1.,
		},

		{
			ErrorProbability: .05,
			ErrorBound:       1.,
		},

		{
			ErrorProbability: .10,
			ErrorBound:       2.,
		},

		{
			ErrorProbability: .20,
			ErrorBound:       2.,
		},
	}

	for _, c := range testCases {
		caseName := fmt.Sprintf("error_prob_%.2f_error_bound_%.0f", c.ErrorProbability, c.ErrorBound)
		t.Run(caseName, func(t *testing.T) {
			cms := NewCMSWithErrorParams(&CountMinSketchCfg{
				ErrorProbability: c.ErrorProbability,
				TotalFreq:        float64(countOfInsertions(maxValue)),
				MaxErr:           c.ErrorBound,
			})

			overEstimated := 0.
			for i := 1; i < maxValue+1; i++ {
				bytesData := makeTestCMSKey(i)

				for k := 0; k < (i - 1); k++ {
					cms.InsertWithCount(bytesData)
				}

				cnt := cms.InsertWithCount(bytesData)
				if cnt != uint32(i) {
					overEstimated++
				}
			}

			errProb := overEstimated / float64(countOfInsertions(maxValue))
			if overEstimated == 0 {
				errProb = 0.
			}
			assert.LessOrEqualf(t, errProb, c.ErrorProbability,
				"%s: error(%.4f) is greater than defined(%.2f)",
				caseName, errProb, c.ErrorProbability)
		})
	}
}

func TestCmsEmpty(t *testing.T) {
	cms := NewCMSWithErrorParams(&CountMinSketchCfg{
		ErrorProbability: .99,
		TotalFreq:        100,
		MaxErr:           1,
	})

	assert.Zero(t, cms.Count([]byte{1}))
	assert.Zero(t, cms.Count([]byte{2}))
	assert.Zero(t, cms.Count([]byte{3}))
}

func TestCmsNonExist(t *testing.T) {
	cms := NewCMSWithErrorParams(&CountMinSketchCfg{
		ErrorProbability: .99,
		TotalFreq:        100,
		MaxErr:           1,
	})

	cms.Insert([]byte{1})
	cms.Insert([]byte{2})
	cms.Insert([]byte{3})

	assert.Zero(t, cms.Count([]byte{4}))
	assert.Zero(t, cms.Count([]byte{5}))
	assert.Zero(t, cms.Count([]byte{6}))
}

func TestCmsInsertSimple(t *testing.T) {
	cms := NewCMSWithErrorParams(&CountMinSketchCfg{
		ErrorProbability: .99,
		TotalFreq:        100,
		MaxErr:           1,
	})

	assert.Equal(t, uint32(1), cms.InsertWithCount([]byte{1}))
	assert.Equal(t, uint32(1), cms.Count([]byte{1}))

	assert.Equal(t, uint32(2), cms.InsertWithCount([]byte{1}))
	assert.Equal(t, uint32(2), cms.Count([]byte{1}))

	assert.Equal(t, uint32(3), cms.InsertWithCount([]byte{1}))
	assert.Equal(t, uint32(3), cms.Count([]byte{1}))
}

func TestCmsClear(t *testing.T) {
	cms := NewCMSWithErrorParams(&CountMinSketchCfg{
		ErrorProbability: 0.001,
		TotalFreq:        100,
		MaxErr:           1,
	})

	cms.Insert([]byte{1})
	assert.Equal(t, uint32(1), cms.Count([]byte{1}))

	cms.Insert([]byte{2})
	cms.Insert([]byte{2})
	assert.Equal(t, uint32(2), cms.Count([]byte{2}))

	cms.Insert([]byte{3})
	cms.Insert([]byte{3})
	cms.Insert([]byte{3})
	assert.Equal(t, uint32(3), cms.Count([]byte{3}))

	cms.Clear()
	assert.Equal(t, uint32(0), cms.Count([]byte{1}))
	assert.Equal(t, uint32(0), cms.Count([]byte{2}))
	assert.Equal(t, uint32(0), cms.Count([]byte{3}))
}
