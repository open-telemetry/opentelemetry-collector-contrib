package pressurescraper

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
)

type pressureResource string

const (
	pressureProcDir                  = "/proc/pressure/"
	cpuResource     pressureResource = "cpu"
	memResource     pressureResource = "memory"
)

func getPressureStats(resource pressureResource) (*PressureResourceStats, error) {
	f, err := os.Open(path.Join(pressureProcDir, string(resource)))
	if err != nil {
		return nil, err
	}

	defer f.Close()

	return parsePressureStats(f)
}

func parsePressureStats(f io.Reader) (*PressureResourceStats, error) {
	resourceStats := &PressureResourceStats{}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)

		if len(fields) < 5 {
			return nil, fmt.Errorf("Invalid line (<5 fields): %v", line)
		}

		var stats PressureStats

		for _, field := range fields[1:] {
			// strings.Cut is the cleanest way to split exactly once
			key, val, found := strings.Cut(field, "=")
			if !found {
				continue
			}

			var err error
			switch key {
			case "avg10":
				stats.avg10, err = strconv.ParseFloat(val, 64)
				if err != nil {
					return nil, err
				}
			case "avg60":
				stats.avg60, err = strconv.ParseFloat(val, 64)
				if err != nil {
					return nil, err
				}
			case "avg300":
				stats.avg300, err = strconv.ParseFloat(val, 64)
				if err != nil {
					return nil, err
				}
			case "total":
				stats.total, err = strconv.ParseUint(val, 10, 64)
				if err != nil {
					return nil, err
				}
			}
		}

		switch fields[0] {
		case "some":
			resourceStats.some = stats
		case "full":
			resourceStats.full = stats
		default:
			continue // Skip lines we don't recognize
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning Linux pressure stats: %w", err)
	}

	return resourceStats, nil
}
