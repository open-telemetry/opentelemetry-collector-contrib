// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cms

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewSlidingCMSInputParamsEmptyBuckets(t *testing.T) {
	sCms, err := NewSlidingCMSWithStartPoint(
		BucketsCfg{
			ObservationInterval: time.Second * 2,
			Buckets:             1,
		},
		CountMinSketchCfg{
			ErrorProbability: .1,
			TotalFreq:        1,
			MaxErr:           1,
		},
		time.Now(),
	)

	assert.NoError(t, err)
	assert.NotNil(t, sCms)

	bCfg := BucketsCfg{
		ObservationInterval: time.Second * 2,
		Buckets:             0,
	}
	cmsCfg := CountMinSketchCfg{
		ErrorProbability: .1,
		TotalFreq:        1,
		MaxErr:           1,
	}
	tm := time.Now()

	sCms, err = NewSlidingCMSWithStartPoint(bCfg, cmsCfg, tm)
	assert.ErrorIs(t, err, errIncorrectBucketsCount)
	assert.Nil(t, sCms)

	sCms, err = NewSlidingPredefinedCMSWithStartPoint(bCfg, make([]CountMinSketch, 1), tm)
	assert.ErrorIs(t, err, errIncorrectBucketsCount)
	assert.Nil(t, sCms)
}

func TestNewSlidingCMSInputParamsNonMultiplyInterval(t *testing.T) {
	cmsCfg := CountMinSketchCfg{
		ErrorProbability: .1,
		TotalFreq:        1,
		MaxErr:           1,
	}
	tm := time.Now()

	testCases := []struct {
		ObservationInterval time.Duration
		Buckets             uint8
	}{
		{
			ObservationInterval: time.Millisecond * 1,
			Buckets:             1,
		},
		{
			ObservationInterval: 1001 * time.Millisecond,
			Buckets:             1,
		},
		{
			ObservationInterval: 1001 * time.Millisecond,
			Buckets:             2,
		},
		{
			ObservationInterval: 100 * time.Millisecond,
			Buckets:             3,
		},
	}

	for _, c := range testCases {
		caseName := fmt.Sprintf(
			"observation_interval_%s_buckets_%d",
			c.ObservationInterval,
			c.Buckets,
		)

		t.Run(caseName, func(t *testing.T) {
			bCfg := BucketsCfg{
				ObservationInterval: c.ObservationInterval,
				Buckets:             c.Buckets,
			}

			sCms, err := NewSlidingCMSWithStartPoint(bCfg, cmsCfg, tm)
			assert.ErrorIs(t, err, errNonMultiplyObservInterval)
			assert.Nil(t, sCms)

			cmsStubs := make([]CountMinSketch, c.Buckets)
			sCms, err = NewSlidingPredefinedCMSWithStartPoint(bCfg, cmsStubs, tm)
			assert.ErrorIs(t, err, errNonMultiplyObservInterval)
			assert.Nil(t, sCms)
		})
	}
}

func TestNewSlidingCMSInputParamsEmptyInterval(t *testing.T) {
	bCfg := BucketsCfg{
		ObservationInterval: 0,
		Buckets:             1,
	}

	cmsCfg := CountMinSketchCfg{
		ErrorProbability: .1,
		TotalFreq:        1,
		MaxErr:           1,
	}

	cmsStubs := []CountMinSketch{&StubCms{}}

	sCms, err := NewSlidingCMSWithStartPoint(bCfg, cmsCfg, time.Now())
	assert.ErrorIs(t, err, errEmptyObservInterval)
	assert.Nil(t, sCms)

	sCms, err = NewSlidingPredefinedCMSWithStartPoint(bCfg, cmsStubs, time.Now())
	assert.ErrorIs(t, err, errEmptyObservInterval)
	assert.Nil(t, sCms)
}

func TestNewPredefinedSlidingCMSIncorrectCmsNum(t *testing.T) {
	bCfg := BucketsCfg{
		ObservationInterval: 2 * time.Second,
		Buckets:             2,
	}

	cmsStubs := []CountMinSketch{&StubCms{}}

	sCms, err := NewSlidingPredefinedCMSWithStartPoint(bCfg, cmsStubs, time.Now())
	assert.ErrorIs(t, err, errIncorrectBucketsCount)
	assert.Nil(t, sCms)
}

func TestNewSlidingCMSUpdateBucketsSimple(t *testing.T) {
	tm := time.Unix(1, 0)

	cmsData := []CountMinSketch{
		NewEmptyCmsStub(1),
		NewEmptyCmsStub(2),
		NewEmptyCmsStub(3),
	}

	bCfg := BucketsCfg{
		ObservationInterval: 3 * time.Second,
		Buckets:             3,
	}

	sCms, _ := NewSlidingPredefinedCMSWithStartPoint(bCfg, cmsData, tm)

	buckets := sCms.Buckets()
	tail, _ := buckets.Tail()
	assert.Equal(t, 1, buckets.Size())
	assert.Equal(t, uint32(3), buckets.Capacity())
	assert.Equal(t, 1, tail.(*StubCms).id)

	sCms.updateBuckets(time.Unix(1, 0))
	buckets = sCms.Buckets()
	tail, _ = buckets.Tail()
	assert.Equal(t, 1, buckets.Size())
	assert.Equal(t, uint32(3), buckets.Capacity())
	assert.Equal(t, 1, tail.(*StubCms).id)

	sCms.updateBuckets(time.Unix(1, 1))
	buckets = sCms.Buckets()
	tail, _ = buckets.Tail()
	assert.Equal(t, 1, buckets.Size())
	assert.Equal(t, uint32(3), buckets.Capacity())
	assert.Equal(t, 1, tail.(*StubCms).id)

	sCms.updateBuckets(time.Unix(2, 1))
	buckets = sCms.Buckets()
	tail, _ = buckets.Tail()
	assert.Equal(t, 2, buckets.Size())
	assert.Equal(t, uint32(3), buckets.Capacity())
	assert.Equal(t, 2, tail.(*StubCms).id)

	sCms.updateBuckets(time.Unix(3, 1))
	buckets = sCms.Buckets()
	tail, _ = buckets.Tail()
	assert.Equal(t, 3, buckets.Size())
	assert.Equal(t, uint32(3), buckets.Capacity())
	assert.Equal(t, 3, tail.(*StubCms).id)
}

func TestNewSlidingCMSUpdateBucketsOverlap(t *testing.T) {
	tm := time.Unix(1, 0)

	cmsData := []CountMinSketch{
		NewEmptyCmsStub(1),
		NewEmptyCmsStub(2),
		NewEmptyCmsStub(3),
	}

	bCfg := BucketsCfg{
		ObservationInterval: 3 * time.Second,
		Buckets:             3,
	}

	sCms, _ := NewSlidingPredefinedCMSWithStartPoint(bCfg, cmsData, tm)

	sCms.updateBuckets(time.Unix(2, 1))
	sCms.updateBuckets(time.Unix(3, 1))
	sCms.updateBuckets(time.Unix(4, 1))

	buckets := sCms.Buckets()
	tail, _ := buckets.Tail()

	assert.Equal(t, 3, buckets.Size())
	assert.Equal(t, uint32(3), buckets.Capacity())
	assert.Equal(t, 1, tail.(*StubCms).id)

	assert.Equal(t, 1, cmsData[0].(*StubCms).ClearCnt)
	assert.Equal(t, 0, cmsData[1].(*StubCms).ClearCnt)
	assert.Equal(t, 0, cmsData[2].(*StubCms).ClearCnt)

	assert.Equal(t, time.Unix(4, 0), sCms.CurrentObservationIntervalStartTm())
}

func TestNewSlidingCMSUpdateBucketsTimeGap(t *testing.T) {
	tm := time.Unix(1, 0)

	cmsData := []CountMinSketch{
		NewEmptyCmsStub(1),
		NewEmptyCmsStub(2),
		NewEmptyCmsStub(3),
	}

	bCfg := BucketsCfg{
		ObservationInterval: 3 * time.Second,
		Buckets:             3,
	}

	sCms, _ := NewSlidingPredefinedCMSWithStartPoint(bCfg, cmsData, tm)

	sCms.updateBuckets(time.Unix(4, 1))
	buckets := sCms.Buckets()
	tail, _ := buckets.Tail()

	assert.Equal(t, 2, buckets.Size())
	assert.Equal(t, uint32(3), buckets.Capacity())
	assert.Equal(t, 2, tail.(*StubCms).id)
	assert.Equal(t, 0, cmsData[0].(*StubCms).ClearCnt)
	assert.Equal(t, 0, cmsData[1].(*StubCms).ClearCnt)
	assert.Equal(t, 0, cmsData[2].(*StubCms).ClearCnt)
	assert.Equal(t, time.Unix(1, 0), sCms.CurrentObservationIntervalStartTm())

	sCms.updateBuckets(time.Unix(5, 1))
	sCms.updateBuckets(time.Unix(10, 1))
	buckets = sCms.Buckets()
	tail, _ = buckets.Tail()

	assert.Equal(t, 3, buckets.Size())
	assert.Equal(t, uint32(3), buckets.Capacity())
	assert.Equal(t, 1, tail.(*StubCms).id)
	assert.Equal(t, 1, cmsData[0].(*StubCms).ClearCnt)
	assert.Equal(t, 0, cmsData[1].(*StubCms).ClearCnt)
	assert.Equal(t, 0, cmsData[2].(*StubCms).ClearCnt)
	assert.Equal(t, time.Unix(10, 0), sCms.CurrentObservationIntervalStartTm())
}

func TestNewSlidingCMSInsertOverlap(t *testing.T) {
	tm := time.Unix(1, 0)
	bCfg := BucketsCfg{
		ObservationInterval: 3 * time.Second,
		Buckets:             3,
		EstimationSoftLimit: 10,
	}
	cmsData := []CountMinSketch{
		NewEmptyCmsStub(1),
		NewEmptyCmsStub(2),
		NewEmptyCmsStub(3),
	}
	cmsKey := []byte(strconv.Itoa(1))

	sCms, _ := NewSlidingPredefinedCMSWithStartPoint(bCfg, cmsData, tm)

	sCms.insertWithTime(cmsKey, time.Unix(1, 1))
	sCms.insertWithTime(cmsKey, time.Unix(2, 1))
	sCms.insertWithTime(cmsKey, time.Unix(3, 1))
	sCms.insertWithTime(cmsKey, time.Unix(4, 1))

	assert.Equal(t, 2, cmsData[0].(*StubCms).InsertionsReq)
	assert.Equal(t, 1, cmsData[1].(*StubCms).InsertionsReq)
	assert.Equal(t, 1, cmsData[2].(*StubCms).InsertionsReq)
}

func TestNewSlidingCMSCountOverlapWithEmptySoftLimit(t *testing.T) {
	tm := time.Unix(1, 0)
	bCfg := BucketsCfg{
		ObservationInterval: 3 * time.Second,
		Buckets:             3,
		EstimationSoftLimit: 0,
	}
	cmsData := []CountMinSketch{
		NewCmsStubWithCounts(1, CntMap{"1": 1}),
		NewCmsStubWithCounts(2, CntMap{"1": 2}),
		NewCmsStubWithCounts(3, CntMap{"1": 3}),
	}
	cmsKey := []byte(strconv.Itoa(1))

	sCms, _ := NewSlidingPredefinedCMSWithStartPoint(bCfg, cmsData, tm)

	sCms.insertWithTime(cmsKey, time.Unix(1, 1))
	assert.Equal(t, uint32(1), sCms.Count(cmsKey))
	assert.Equal(t, 1, cmsData[0].(*StubCms).CountReq)

	sCms.insertWithTime(cmsKey, time.Unix(2, 1))
	assert.Equal(t, uint32(3), sCms.Count(cmsKey))
	assert.Equal(t, 2, cmsData[0].(*StubCms).CountReq)
	assert.Equal(t, 1, cmsData[1].(*StubCms).CountReq)

	sCms.insertWithTime(cmsKey, time.Unix(3, 1))
	assert.Equal(t, uint32(6), sCms.Count(cmsKey))
	assert.Equal(t, 3, cmsData[0].(*StubCms).CountReq)
	assert.Equal(t, 2, cmsData[1].(*StubCms).CountReq)
	assert.Equal(t, 1, cmsData[2].(*StubCms).CountReq)

	sCms.insertWithTime(cmsKey, time.Unix(4, 1))
	assert.Equal(t, uint32(6), sCms.Count(cmsKey))
	assert.Equal(t, 4, cmsData[0].(*StubCms).CountReq)
	assert.Equal(t, 3, cmsData[1].(*StubCms).CountReq)
	assert.Equal(t, 2, cmsData[2].(*StubCms).CountReq)
}

func TestNewSlidingCMSInsertWithCountOverlap(t *testing.T) {
	tm := time.Unix(1, 0)
	bCfg := BucketsCfg{
		ObservationInterval: 3 * time.Second,
		Buckets:             3,
		EstimationSoftLimit: 10,
	}
	cmsData := []CountMinSketch{
		NewCmsStubWithCounts(1, CntMap{"1": 1}),
		NewCmsStubWithCounts(2, CntMap{"1": 2}),
		NewCmsStubWithCounts(3, CntMap{"1": 3}),
	}
	cmsKey := []byte(strconv.Itoa(1))

	sCms, _ := NewSlidingPredefinedCMSWithStartPoint(bCfg, cmsData, tm)

	assert.Equal(t, uint32(1), sCms.timedInsertWithCount(cmsKey, time.Unix(1, 1)))
	assert.Equal(t, 0, cmsData[0].(*StubCms).CountReq)
	assert.Equal(t, 1, cmsData[0].(*StubCms).InsertionsWithCnt)

	assert.Equal(t, uint32(3), sCms.timedInsertWithCount(cmsKey, time.Unix(2, 1)))
	assert.Equal(t, 1, cmsData[0].(*StubCms).CountReq)
	assert.Equal(t, 0, cmsData[1].(*StubCms).CountReq)

	assert.Equal(t, 1, cmsData[0].(*StubCms).InsertionsWithCnt)
	assert.Equal(t, 1, cmsData[1].(*StubCms).InsertionsWithCnt)

	assert.Equal(t, uint32(6), sCms.timedInsertWithCount(cmsKey, time.Unix(3, 1)))
	assert.Equal(t, 2, cmsData[0].(*StubCms).CountReq)
	assert.Equal(t, 1, cmsData[1].(*StubCms).CountReq)
	assert.Equal(t, 0, cmsData[2].(*StubCms).CountReq)

	assert.Equal(t, 1, cmsData[0].(*StubCms).InsertionsWithCnt)
	assert.Equal(t, 1, cmsData[1].(*StubCms).InsertionsWithCnt)
	assert.Equal(t, 1, cmsData[2].(*StubCms).InsertionsWithCnt)

	assert.Equal(t, uint32(6), sCms.timedInsertWithCount(cmsKey, time.Unix(4, 1)))
	assert.Equal(t, 2, cmsData[0].(*StubCms).CountReq)
	assert.Equal(t, 2, cmsData[1].(*StubCms).CountReq)
	assert.Equal(t, 1, cmsData[2].(*StubCms).CountReq)

	assert.Equal(t, 2, cmsData[0].(*StubCms).InsertionsWithCnt)
	assert.Equal(t, 1, cmsData[1].(*StubCms).InsertionsWithCnt)
	assert.Equal(t, 1, cmsData[2].(*StubCms).InsertionsWithCnt)
}

func TestNewSlidingCMSCountWithExactSoftLimit(t *testing.T) {
	tm := time.Unix(1, 0)
	bCfg := BucketsCfg{
		ObservationInterval: 3 * time.Second,
		Buckets:             3,
		EstimationSoftLimit: 2,
	}
	cmsData := []CountMinSketch{
		NewCmsStubWithCounts(1, CntMap{"1": 1}),
		NewCmsStubWithCounts(2, CntMap{"1": 2}),
		NewCmsStubWithCounts(3, CntMap{"1": 3}),
	}
	cmsKey := []byte(strconv.Itoa(1))

	testCases := []struct {
		Name        string
		SoftLimit   uint32
		NumOfProbes int
	}{
		{
			Name:        "exact_soft_limit_1",
			SoftLimit:   1,
			NumOfProbes: 1,
		},

		{
			Name:        "exact_soft_limit_2",
			SoftLimit:   3,
			NumOfProbes: 2,
		},

		{
			Name:        "exact_soft_limit_3",
			SoftLimit:   6,
			NumOfProbes: 3,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			bCfg.EstimationSoftLimit = tc.SoftLimit
			testCms := CopyCmsStubSlice(cmsData)

			sCms, _ := NewSlidingPredefinedCMSWithStartPoint(bCfg, testCms, tm)
			sCms.updateBuckets(time.Unix(2, 1))
			sCms.updateBuckets(time.Unix(3, 1))

			assert.Equal(t, tc.SoftLimit, sCms.Count(cmsKey))

			for i := 0; i < tc.NumOfProbes; i++ {
				assert.Equal(t, 1, testCms[i].(*StubCms).CountReq)
			}
			for i := tc.NumOfProbes; i < len(cmsData); i++ {
				assert.Equal(t, 0, testCms[i].(*StubCms).CountReq)
			}
		})
	}
}

func TestNewSlidingCMSCountWithSoftLimitOverflow(t *testing.T) {
	tm := time.Unix(1, 0)
	bCfg := BucketsCfg{
		ObservationInterval: 3 * time.Second,
		Buckets:             3,
		EstimationSoftLimit: 2,
	}
	cmsData := []CountMinSketch{
		NewCmsStubWithCounts(1, CntMap{"1": 2}),
		NewCmsStubWithCounts(2, CntMap{"1": 3}),
		NewCmsStubWithCounts(3, CntMap{"1": 4}),
	}
	cmsKey := []byte(strconv.Itoa(1))

	testCases := []struct {
		Name          string
		SoftLimit     uint32
		NumOfProbes   int
		ExpectedCount uint32
	}{
		{
			Name:          "soft_limit_1",
			SoftLimit:     1,
			NumOfProbes:   1,
			ExpectedCount: 2,
		},

		{
			Name:          "soft_limit_2",
			SoftLimit:     3,
			NumOfProbes:   2,
			ExpectedCount: 5,
		},

		{
			Name:          "soft_limit_3",
			SoftLimit:     6,
			NumOfProbes:   3,
			ExpectedCount: 9,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			bCfg.EstimationSoftLimit = tc.SoftLimit
			testCms := CopyCmsStubSlice(cmsData)

			sCms, _ := NewSlidingPredefinedCMSWithStartPoint(bCfg, testCms, tm)
			sCms.updateBuckets(time.Unix(2, 1))
			sCms.updateBuckets(time.Unix(3, 1))

			assert.Equal(t, tc.ExpectedCount, sCms.Count(cmsKey))

			for i := 0; i < tc.NumOfProbes; i++ {
				assert.Equal(t, 1, testCms[i].(*StubCms).CountReq)
			}
			for i := tc.NumOfProbes; i < len(cmsData); i++ {
				assert.Equal(t, 0, testCms[i].(*StubCms).CountReq)
			}
		})
	}
}

func TestNewSlidingCMSInsertCountWithSoftLimitOverflow(t *testing.T) {
	tm := time.Unix(1, 0)
	bCfg := BucketsCfg{
		ObservationInterval: 3 * time.Second,
		Buckets:             3,
		EstimationSoftLimit: 2,
	}
	cmsData := []CountMinSketch{
		NewCmsStubWithCounts(1, CntMap{"1": 2}),
		NewCmsStubWithCounts(2, CntMap{"1": 3}),
		NewCmsStubWithCounts(3, CntMap{"1": 4}),
	}
	cmsKey := []byte(strconv.Itoa(1))

	testCases := []struct {
		Name          string
		SoftLimit     uint32
		NumOfProbes   int
		ExpectedCount uint32
	}{
		{
			Name:          "soft_limit_1",
			SoftLimit:     3,
			NumOfProbes:   0,
			ExpectedCount: 4,
		},

		{
			Name:          "soft_limit_2",
			SoftLimit:     5,
			NumOfProbes:   1,
			ExpectedCount: 6,
		},

		{
			Name:          "soft_limit_3",
			SoftLimit:     8,
			NumOfProbes:   2,
			ExpectedCount: 9,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			bCfg.EstimationSoftLimit = tc.SoftLimit
			testCms := CopyCmsStubSlice(cmsData)

			sCms, _ := NewSlidingPredefinedCMSWithStartPoint(bCfg, testCms, tm)
			sCms.updateBuckets(time.Unix(2, 1))
			sCms.updateBuckets(time.Unix(3, 1))

			assert.Equal(t, tc.ExpectedCount, sCms.timedInsertWithCount(cmsKey, time.Unix(3, 1)))
			tail, _ := sCms.Buckets().Tail()
			assert.Equal(t, 1, tail.(*StubCms).InsertionsWithCnt)

			for i := 0; i < tc.NumOfProbes; i++ {
				assert.Equal(t, 1, testCms[i].(*StubCms).CountReq)
			}
			for i := tc.NumOfProbes; i < len(cmsData); i++ {
				assert.Equal(t, 0, testCms[i].(*StubCms).CountReq)
			}
		})
	}
}

func TestNewSlidingCMSInsertCountWithExactSoftLimit(t *testing.T) {
	tm := time.Unix(1, 0)
	bCfg := BucketsCfg{
		ObservationInterval: 3 * time.Second,
		Buckets:             3,
		EstimationSoftLimit: 2,
	}
	cmsData := []CountMinSketch{
		NewCmsStubWithCounts(1, CntMap{"1": 2}),
		NewCmsStubWithCounts(2, CntMap{"1": 3}),
		NewCmsStubWithCounts(3, CntMap{"1": 4}),
	}
	cmsKey := []byte(strconv.Itoa(1))

	testCases := []struct {
		Name          string
		SoftLimit     uint32
		NumOfProbes   int
		ExpectedCount uint32
	}{
		{
			Name:          "soft_limit_1",
			SoftLimit:     4,
			NumOfProbes:   0,
			ExpectedCount: 4,
		},

		{
			Name:          "soft_limit_2",
			SoftLimit:     6,
			NumOfProbes:   1,
			ExpectedCount: 6,
		},

		{
			Name:          "soft_limit_3",
			SoftLimit:     8,
			NumOfProbes:   2,
			ExpectedCount: 9,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			bCfg.EstimationSoftLimit = tc.SoftLimit
			testCms := CopyCmsStubSlice(cmsData)

			sCms, _ := NewSlidingPredefinedCMSWithStartPoint(bCfg, testCms, tm)
			sCms.updateBuckets(time.Unix(2, 1))
			sCms.updateBuckets(time.Unix(3, 1))

			assert.Equal(t, tc.ExpectedCount, sCms.timedInsertWithCount(cmsKey, time.Unix(3, 1)))
			tail, _ := sCms.Buckets().Tail()
			assert.Equal(t, 1, tail.(*StubCms).InsertionsWithCnt)

			for i := 0; i < tc.NumOfProbes; i++ {
				assert.Equal(t, 1, testCms[i].(*StubCms).CountReq)
			}
			for i := tc.NumOfProbes; i < len(cmsData); i++ {
				assert.Equal(t, 0, testCms[i].(*StubCms).CountReq)
			}
		})
	}
}

func TestNewSlidingCMSClearSimple(t *testing.T) {
	tm := time.Unix(1, 0)
	bCfg := BucketsCfg{
		ObservationInterval: 3 * time.Second,
		Buckets:             3,
		EstimationSoftLimit: 2,
	}
	cmsData := []CountMinSketch{
		NewEmptyCmsStub(1),
		NewEmptyCmsStub(2),
		NewEmptyCmsStub(3),
	}

	sCms, _ := NewSlidingPredefinedCMSWithStartPoint(bCfg, cmsData, tm)

	assert.Zero(t, cmsData[0].(*StubCms).ClearCnt)
	assert.Zero(t, cmsData[1].(*StubCms).ClearCnt)
	assert.Zero(t, cmsData[2].(*StubCms).ClearCnt)

	sCms.Clear()
	assert.Equal(t, 1, cmsData[0].(*StubCms).ClearCnt)
	assert.Equal(t, 0, cmsData[1].(*StubCms).ClearCnt)
	assert.Equal(t, 0, cmsData[2].(*StubCms).ClearCnt)

	sCms.updateBuckets(time.Unix(2, 1))
	sCms.Clear()
	assert.Equal(t, 2, cmsData[0].(*StubCms).ClearCnt)
	assert.Equal(t, 1, cmsData[1].(*StubCms).ClearCnt)
	assert.Equal(t, 0, cmsData[2].(*StubCms).ClearCnt)
}

func TestNewSlidingCMSClearOverflow(t *testing.T) {
	tm := time.Unix(1, 0)
	bCfg := BucketsCfg{
		ObservationInterval: 3 * time.Second,
		Buckets:             3,
		EstimationSoftLimit: 2,
	}
	cmsData := []CountMinSketch{
		NewEmptyCmsStub(1),
		NewEmptyCmsStub(2),
		NewEmptyCmsStub(3),
	}

	sCms, _ := NewSlidingPredefinedCMSWithStartPoint(bCfg, cmsData, tm)

	assert.Zero(t, cmsData[0].(*StubCms).ClearCnt)
	assert.Zero(t, cmsData[1].(*StubCms).ClearCnt)
	assert.Zero(t, cmsData[2].(*StubCms).ClearCnt)

	sCms.updateBuckets(time.Unix(2, 1))
	sCms.updateBuckets(time.Unix(3, 1))
	sCms.Clear()
	assert.Equal(t, 1, cmsData[0].(*StubCms).ClearCnt)
	assert.Equal(t, 1, cmsData[1].(*StubCms).ClearCnt)
	assert.Equal(t, 1, cmsData[2].(*StubCms).ClearCnt)

	sCms.updateBuckets(time.Unix(4, 1))
	assert.Equal(t, 2, cmsData[0].(*StubCms).ClearCnt)
	assert.Equal(t, 1, cmsData[1].(*StubCms).ClearCnt)
	assert.Equal(t, 1, cmsData[2].(*StubCms).ClearCnt)

	sCms.Clear()
	assert.Equal(t, 3, cmsData[0].(*StubCms).ClearCnt)
	assert.Equal(t, 2, cmsData[1].(*StubCms).ClearCnt)
	assert.Equal(t, 2, cmsData[2].(*StubCms).ClearCnt)
}
