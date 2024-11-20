// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cms // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/cms"

import (
	"math"
)

// CountMinSketch interface for Count-Min Sketch data structure. CMS is a
// probabilistic data structure that provides an estimate of the frequency
// of elements in a data stream.
type CountMinSketch interface {
	// Count returns the estimated frequency of the given key in the data
	// stream.
	Count(key []byte) uint32
	// Insert increments the counters in the Count-Min Sketch for the given
	// key.
	Insert(element []byte)
	// Clear resets the internal state of Count-Min Sketch
	Clear()
	// InsertWithCount increases the count of specified key and	returns a new
	// estimated frequency of the key.
	InsertWithCount(element []byte) uint32
}

type hasher interface {
	Hash([]byte) uint32
}

// CountMinSketchCfg describes the main configuration options for CMS data
// structure
type CountMinSketchCfg struct {
	// MaxErr approximation error, determines the error bound of the frequency
	// estimates. Used together with the TotalFreq option to calculate the
	// epsilon (ε) parameter for the CountMin Sketch.
	MaxErr float64
	// ErrorProbability error probability (δ, delta), defines the probability that
	// the error exceeds the bound.
	ErrorProbability float64
	// TotalFreq total number of elements (keys) in the data stream. Used in
	// calculation of epsilon (ε) parameter for the CountMin Sketch
	TotalFreq float64
}

type CMS struct {
	data [][]uint32
	hs   []hasher
}

// NewCMS creates new CMS structure based on given width and depth.
func NewCMS(w, h int) *CMS {
	data := make([][]uint32, h)
	hs := make([]hasher, h)
	for i := 0; i < h; i++ {
		hs[i] = NewHWHasher(uint32(w), i)
		data[i] = make([]uint32, w)
	}
	return &CMS{
		data: data,
		hs:   hs,
	}
}

// NewCMSWithErrorParams creates new CMS structure based on given config.
// There CMS width = ⌈e/ε⌉, and depth = ⌈ln(1/δ)⌉
func NewCMSWithErrorParams(cfg *CountMinSketchCfg) *CMS {
	d := math.Ceil(math.Log2(1 / cfg.ErrorProbability))
	w := math.Ceil(math.E / (cfg.MaxErr / cfg.TotalFreq))
	return NewCMS(int(w), int(d))
}

// Insert inserts new element in CMS
func (c *CMS) Insert(element []byte) {
	for i, h := range c.hs {
		c.data[i][h.Hash(element)]++
	}
}

// Clear resets the CMS state
func (c *CMS) Clear() {
	for i := range c.hs {
		for k := range c.data[i] {
			c.data[i][k] = 0
		}
	}
}

// Count estimates the frequency of a given element
func (c *CMS) Count(element []byte) uint32 {
	var m uint32 = math.MaxUint32
	for i, h := range c.hs {
		m = min(m, c.data[i][h.Hash(element)])
	}
	return m
}

// InsertWithCount inserts the element to the CMS and returns the element's
// frequency estimation. This method is equivalent to sequential calls to
// Insert(element) and Count(element). However, in comparison with Count+Insert,
// the InsertWithCount method has 2 times less number of hash calculations.
func (c *CMS) InsertWithCount(element []byte) uint32 {
	var m uint32 = math.MaxUint32
	for i, h := range c.hs {
		position := h.Hash(element)
		c.data[i][position]++
		m = min(m, c.data[i][position])
	}
	return m
}
