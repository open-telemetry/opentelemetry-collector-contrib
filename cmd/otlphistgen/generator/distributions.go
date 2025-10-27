// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"math"
	rand "math/rand/v2"
	"time"
)

// Distribution functions for generating statistical data

func ExponentialRandom(rnd *rand.Rand, rate float64) float64 {
	return -math.Log(1.0-rnd.Float64()) / rate
}

func NormalRandom(rnd *rand.Rand, mean, stddev float64) float64 {
	return rnd.NormFloat64()*stddev + mean
}

func LogNormalRandom(rnd *rand.Rand, mu, sigma float64) float64 {
	return math.Exp(NormalRandom(rnd, mu, sigma))
}

func WeibullRandom(rnd *rand.Rand, shape, scale float64) float64 {
	return scale * math.Pow(-math.Log(1.0-rnd.Float64()), 1.0/shape)
}

func BetaRandom(rnd *rand.Rand, alpha, beta float64) float64 {
	x := GammaRandom(rnd, alpha, 1.0)
	y := GammaRandom(rnd, beta, 1.0)
	return x / (x + y)
}

func GammaRandom(rnd *rand.Rand, alpha, beta float64) float64 {
	if alpha < 1.0 {
		// Use Johnk's generator for alpha < 1
		for {
			u := rnd.Float64()
			v := rnd.Float64()
			x := math.Pow(u, 1.0/alpha)
			y := math.Pow(v, 1.0/(1.0-alpha))
			if x+y <= 1.0 {
				if x+y > 0 {
					return beta * x / (x + y) * (-math.Log(rnd.Float64()))
				}
			}
		}
	}

	// Marsaglia and Tsang's method for alpha >= 1
	d := alpha - 1.0/3.0
	c := 1.0 / math.Sqrt(9.0*d)

	for {
		x := rnd.NormFloat64()
		v := 1.0 + c*x
		if v <= 0 {
			continue
		}
		v = v * v * v
		u := rnd.Float64()
		if u < 1.0-0.0331*(x*x)*(x*x) {
			return beta * d * v
		}
		if math.Log(u) < 0.5*x*x+d*(1.0-v+math.Log(v)) {
			return beta * d * v
		}
	}
}

// Time-based value functions

func SinusoidalValue(rnd *rand.Rand, timestamp time.Time, amplitude, period, phase, baseline float64) float64 {
	t := float64(timestamp.Unix())
	noise := rnd.NormFloat64() * amplitude * 0.1 // 10% noise
	return baseline + amplitude*math.Sin(2*math.Pi*t/period+phase) + noise
}

func SpikyValue(rnd *rand.Rand, baseline, spikeHeight, spikeProb float64) float64 {
	if rnd.Float64() < spikeProb {
		return baseline + spikeHeight*rnd.Float64()
	}
	return baseline + rnd.NormFloat64()*baseline*0.1
}

func TrendingValue(rnd *rand.Rand, timestamp time.Time, startValue, trendRate, noise float64) float64 {
	t := float64(timestamp.Unix())
	trend := startValue + trendRate*t
	return trend + rnd.NormFloat64()*noise
}
