package dockerstatsreceiver

import (
	"testing"

	ctypes "github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
)

func Test_calculateCPULimit1(t *testing.T) {
	tests := []struct {
		name string
		args *ctypes.HostConfig
		want float64
	}{
		{
			"Test CPULimit",
			&ctypes.HostConfig{
				Resources: ctypes.Resources{
					NanoCPUs: 2500000000,
				},
			},
			2.5,
		},
		{
			"Test CPUSetCpu",
			&ctypes.HostConfig{
				Resources: ctypes.Resources{
					CpusetCpus: "0-2",
				},
			},
			3,
		},
		{
			"Test CPUQuota",
			&ctypes.HostConfig{
				Resources: ctypes.Resources{
					CPUQuota: 50000,
				},
			},
			0.5,
		},
		{
			"Test CPUQuota Custom Period",
			&ctypes.HostConfig{
				Resources: ctypes.Resources{
					CPUQuota:  300000,
					CPUPeriod: 200000,
				},
			},
			1.5,
		},
		{
			"Test Default",
			&ctypes.HostConfig{
				Resources: ctypes.Resources{
					NanoCPUs:   1800000000,
					CpusetCpus: "0-1",
					CPUQuota:   400000,
				},
			},
			1.8,
		},
		{
			"Test No Values",
			&ctypes.HostConfig{
				Resources: ctypes.Resources{},
			},
			0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, calculateCPULimit(tt.args), "calculateCPULimit(%v)", tt.args)
		})
	}
}

func Test_parseCPUSet(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"0,2", 2},
		{"0-2", 3},
		{"0-2,4", 4},
		{"0-2,4-5", 5},
		{"", 1},
	}

	for _, test := range tests {
		result := parseCPUSet(test.input)
		if result != test.expected {
			t.Errorf("Expected parseCPUSet(%s) to be %f, but got %f", test.input, test.expected, result)
		}
	}
}
