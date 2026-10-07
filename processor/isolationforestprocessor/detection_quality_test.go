// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// detection_quality_test.go - Detection-quality tests for the isolation forest.
//
// These tests assert that the detector discriminates labeled anomalies from
// normal samples, not only that it runs. They encode two shipped failure modes
// as regressions: scores that were identical for all inputs (#46988), and a
// contamination_rate parameter that had no effect on flagging (#47115). They
// also compare the operating point against the predict-all F1 baseline
// (2p/(1+p) at anomaly prevalence p), below which a detector that flags
// everything scores the same as one that detects.
package isolationforestprocessor

import (
	"math"
	"math/rand/v2"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// genCluster returns n 2-D samples drawn from a normal distribution centered
// at (cx, cy) with standard deviation sigma, using a fixed-seed generator so
// the fixture is identical across runs.
func genCluster(rng *rand.Rand, n int, cx, cy, sigma float64) [][]float64 {
	samples := make([][]float64, n)
	for i := range samples {
		samples[i] = []float64{
			cx + rng.NormFloat64()*sigma,
			cy + rng.NormFloat64()*sigma,
		}
	}
	return samples
}

// labeledFixture returns a training stream of normal samples plus a held-out
// labeled evaluation set. Normal data clusters around (1, 1). Anomalies are
// drawn from clusters far outside the normal region, so any detector with
// discriminative power separates them by a wide margin.
func labeledFixture() (train, evalNormal, evalAnomaly [][]float64) {
	rng := rand.New(rand.NewPCG(42, 1337))
	train = genCluster(rng, 800, 1.0, 1.0, 0.15)
	evalNormal = genCluster(rng, 240, 1.0, 1.0, 0.15)
	centers := [][2]float64{{8, 8}, {-4, -4}, {0, 30}, {15, 0}}
	for _, c := range centers {
		evalAnomaly = append(evalAnomaly, genCluster(rng, 15, c[0], c[1], 0.5)...)
	}
	return train, evalNormal, evalAnomaly
}

// trainedForest returns a forest that has processed the fixture's training
// stream. Training uses ProcessSample so the sliding window, trees, and
// adaptive threshold all reflect the normal profile.
func trainedForest(t *testing.T, contaminationRate float64) *onlineIsolationForest {
	t.Helper()
	forest := newOnlineIsolationForest(50, 256, 8, contaminationRate, 10)
	train, _, _ := labeledFixture()
	for _, s := range train {
		forest.ProcessSample(s)
	}
	return forest
}

// rankAUC computes the area under the ROC curve from score ranks, with tie
// correction. 0.5 is chance. It makes no assumption about score scale or
// threshold, which is what makes it suitable for asserting discrimination.
func rankAUC(normalScores, anomalyScores []float64) float64 {
	type scored struct {
		score   float64
		anomaly bool
	}
	all := make([]scored, 0, len(normalScores)+len(anomalyScores))
	for _, s := range normalScores {
		all = append(all, scored{s, false})
	}
	for _, s := range anomalyScores {
		all = append(all, scored{s, true})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].score < all[j].score })

	// Assign mid-ranks to ties.
	ranks := make([]float64, len(all))
	for i := 0; i < len(all); {
		j := i
		for j < len(all) && all[j].score == all[i].score {
			j++
		}
		mid := float64(i+j+1) / 2.0
		for k := i; k < j; k++ {
			ranks[k] = mid
		}
		i = j
	}
	rankSum := 0.0
	for i, s := range all {
		if s.anomaly {
			rankSum += ranks[i]
		}
	}
	nPos := float64(len(anomalyScores))
	nNeg := float64(len(normalScores))
	return (rankSum - nPos*(nPos+1)/2.0) / (nPos * nNeg)
}

// scoreSet scores samples without mutating the forest, so evaluation does not
// train on the evaluation set.
func scoreSet(forest *onlineIsolationForest, samples [][]float64) []float64 {
	scores := make([]float64, len(samples))
	for i, s := range samples {
		scores[i] = forest.calculateAnomalyScore(s)
	}
	return scores
}

// TestDetectionDiscriminationAboveChance trains on normal data and requires
// the forest to rank held-out labeled anomalies above held-out normal samples.
// A detector with no discriminative power scores AUC 0.5 here regardless of
// its score scale, as the constant-score baseline in this test demonstrates.
func TestDetectionDiscriminationAboveChance(t *testing.T) {
	forest := trainedForest(t, 0.1)
	_, evalNormal, evalAnomaly := labeledFixture()

	auc := rankAUC(scoreSet(forest, evalNormal), scoreSet(forest, evalAnomaly))

	// Constant scores must sit at exactly 0.5 under the tie-corrected AUC,
	// which anchors the floor the forest is asserted against.
	constNormal := make([]float64, len(evalNormal))
	constAnomaly := make([]float64, len(evalAnomaly))
	for i := range constNormal {
		constNormal[i] = 0.5
	}
	for i := range constAnomaly {
		constAnomaly[i] = 0.5
	}
	assert.InDelta(t, 0.5, rankAUC(constNormal, constAnomaly), 1e-9)

	require.GreaterOrEqual(t, auc, 0.70,
		"forest AUC %.3f on a widely separated fixture; a detector below this floor is not discriminating", auc)
}

// TestScoresDifferAcrossDistinctInputs is the regression for #46988, where a
// released build produced the same score for every input and no test caught
// it. Widely separated inputs must not receive one identical score.
func TestScoresDifferAcrossDistinctInputs(t *testing.T) {
	forest := trainedForest(t, 0.1)
	probes := [][]float64{{1.0, 1.0}, {8.0, 8.0}, {-4.0, -4.0}, {0.0, 30.0}, {15.0, 0.0}}

	scores := scoreSet(forest, probes)
	minScore, maxScore := scores[0], scores[0]
	for _, s := range scores[1:] {
		minScore = math.Min(minScore, s)
		maxScore = math.Max(maxScore, s)
	}
	require.Greater(t, maxScore-minScore, 1e-6,
		"scores are constant across widely separated inputs (scores: %v); see #46988", scores)
}

// TestContaminationRateDrivesFlagRate is the regression for #47115, where
// contamination_rate was accepted but ignored. The parameter sets the
// score-history percentile used as the flagging threshold, so a materially
// higher contamination rate must flag a larger fraction of the same stream.
func TestContaminationRateDrivesFlagRate(t *testing.T) {
	flaggedFraction := func(contaminationRate float64) float64 {
		forest := trainedForest(t, contaminationRate)
		_, evalNormal, evalAnomaly := labeledFixture()
		flagged, total := 0, 0
		score := func(samples [][]float64) {
			for _, s := range samples {
				forest.thresholdMutex.RLock()
				threshold := forest.threshold
				forest.thresholdMutex.RUnlock()
				if forest.calculateAnomalyScore(s) > threshold {
					flagged++
				}
				total++
			}
		}
		score(evalNormal)
		score(evalAnomaly)
		return float64(flagged) / float64(total)
	}

	low := flaggedFraction(0.02)
	high := flaggedFraction(0.30)
	require.Greater(t, high, low,
		"contamination_rate has no effect on the flagged fraction (0.02 -> %.3f, 0.30 -> %.3f); see #47115", low, high)
}

// TestOperatingPointBeatsPredictAllBaseline compares the detector's F1 at its
// own threshold against the predict-all baseline 2p/(1+p) at the evaluation
// set's anomaly prevalence p. Any score at or below that baseline is
// achievable by flagging every sample, so it is the floor a reported
// operating point has to clear before it means anything.
func TestOperatingPointBeatsPredictAllBaseline(t *testing.T) {
	forest := trainedForest(t, 0.1)
	_, evalNormal, evalAnomaly := labeledFixture()

	forest.thresholdMutex.RLock()
	threshold := forest.threshold
	forest.thresholdMutex.RUnlock()

	tp, fp, fn := 0, 0, 0
	for _, s := range evalNormal {
		if forest.calculateAnomalyScore(s) > threshold {
			fp++
		}
	}
	for _, s := range evalAnomaly {
		if forest.calculateAnomalyScore(s) > threshold {
			tp++
		} else {
			fn++
		}
	}
	require.Positive(t, tp+fp, "detector flagged nothing at its own threshold")
	precision := float64(tp) / float64(tp+fp)
	recall := float64(tp) / float64(tp+fn)
	f1 := 0.0
	if precision+recall > 0 {
		f1 = 2 * precision * recall / (precision + recall)
	}

	p := float64(len(evalAnomaly)) / float64(len(evalAnomaly)+len(evalNormal))
	predictAllF1 := 2 * p / (1 + p)

	require.Greater(t, f1, predictAllF1,
		"F1 %.3f does not beat the predict-all baseline %.3f at prevalence %.3f (precision %.3f, recall %.3f)",
		f1, predictAllF1, p, precision, recall)
}
