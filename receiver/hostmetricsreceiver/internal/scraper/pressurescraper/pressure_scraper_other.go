//go:build !linux

package pressurescraper

func getPressureStats() (*PressureStats, error) {
	return nil, nil
}
