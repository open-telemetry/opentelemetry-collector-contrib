package pressurescraper

type PressureResourceStats struct {
	some PressureStats
	full PressureStats
}

type PressureStats struct {
	avg10  float64
	avg60  float64
	avg300 float64
	total  uint64
}
