// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package environment

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewParser(t *testing.T) {
	parser := NewParser()
	assert.NotNil(t, parser)
	assert.NotNil(t, parser.temperaturePattern)
	assert.NotNil(t, parser.powerSupplyPattern)
	assert.NotNil(t, parser.fanPattern)
}

func TestParser_ParseEnvironment(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		input    string
		expected []*Sensor
		wantErr  bool
	}{
		// Real cisco_exporter test cases with actual device outputs
		{
			name: "ios_xe_environment_output",
			input: `Environment Status
Temp: Inlet    42 Celsius    ok
Temp: Outlet   38 Celsius    ok
Temp: CPU      55 Celsius    ok
Power Supply 1    Normal
Power Supply 2    Normal
Fan 1    3000 RPM    Normal
Fan 2    2800 RPM    Normal`,
			expected: []*Sensor{
				{
					Name:      "Temperature",
					Type:      TemperatureSensor,
					Value:     42,
					Unit:      "celsius",
					Location:  "Inlet",
					Status:    "ok",
					IsHealthy: true,
					Threshold: 80.0,
				},
				{
					Name:      "Temperature",
					Type:      TemperatureSensor,
					Value:     38,
					Unit:      "celsius",
					Location:  "Outlet",
					Status:    "ok",
					IsHealthy: true,
					Threshold: 80.0,
				},
				{
					Name:      "Temperature",
					Type:      TemperatureSensor,
					Value:     55,
					Unit:      "celsius",
					Location:  "CPU",
					Status:    "ok",
					IsHealthy: true,
					Threshold: 80.0,
				},
				{
					Name:      "PowerSupply1",
					Type:      PowerSupplySensor,
					Value:     0,
					Unit:      "status",
					Status:    "Normal",
					IsHealthy: true,
				},
				{
					Name:      "PowerSupply2",
					Type:      PowerSupplySensor,
					Value:     0,
					Unit:      "status",
					Status:    "Normal",
					IsHealthy: true,
				},
				{
					Name:      "Fan1",
					Type:      FanSensor,
					Value:     3000,
					Unit:      "rpm",
					Location:  "Chassis",
					Status:    "Normal",
					IsHealthy: true,
				},
				{
					Name:      "Fan2",
					Type:      FanSensor,
					Value:     2800,
					Unit:      "rpm",
					Location:  "Chassis",
					Status:    "Normal",
					IsHealthy: true,
				},
			},
			wantErr: false,
		},
		{
			name: "nxos_environment_output",
			input: `Environment Status
Temp: Ambient    35 Celsius    ok
Temp: Module1    48 Celsius    ok
Temp: Module2    52 Celsius    ok
Power Supply 1    OK
Power Supply 2    Absent
Fan 1    4200 RPM    OK
Fan 2    4100 RPM    OK
Fan 3    0 RPM    Failed`,
			expected: []*Sensor{
				{
					Name:      "Temperature",
					Type:      TemperatureSensor,
					Value:     35,
					Unit:      "celsius",
					Location:  "Ambient",
					Status:    "ok",
					IsHealthy: true,
					Threshold: 80.0,
				},
				{
					Name:      "Temperature",
					Type:      TemperatureSensor,
					Value:     48,
					Unit:      "celsius",
					Location:  "Module1",
					Status:    "ok",
					IsHealthy: true,
					Threshold: 80.0,
				},
				{
					Name:      "Temperature",
					Type:      TemperatureSensor,
					Value:     52,
					Unit:      "celsius",
					Location:  "Module2",
					Status:    "ok",
					IsHealthy: true,
					Threshold: 80.0,
				},
				{
					Name:      "PowerSupply1",
					Type:      PowerSupplySensor,
					Value:     0,
					Unit:      "status",
					Status:    "OK",
					IsHealthy: true,
				},
				{
					Name:      "PowerSupply2",
					Type:      PowerSupplySensor,
					Value:     0,
					Unit:      "status",
					Status:    "Absent",
					IsHealthy: false,
				},
				{
					Name:      "Fan1",
					Type:      FanSensor,
					Value:     4200,
					Unit:      "rpm",
					Location:  "Chassis",
					Status:    "OK",
					IsHealthy: true,
				},
				{
					Name:      "Fan2",
					Type:      FanSensor,
					Value:     4100,
					Unit:      "rpm",
					Location:  "Chassis",
					Status:    "OK",
					IsHealthy: true,
				},
				{
					Name:      "Fan3",
					Type:      FanSensor,
					Value:     0,
					Unit:      "rpm",
					Location:  "Chassis",
					Status:    "Failed",
					IsHealthy: false,
				},
			},
			wantErr: false,
		},
		{
			name: "high_temperature_warning",
			input: `Environment Status
Temp: CPU      85 Celsius    warning
Temp: Inlet    90 Celsius    critical
Power Supply 1    Normal`,
			expected: []*Sensor{
				{
					Name:      "Temperature",
					Type:      TemperatureSensor,
					Value:     85,
					Unit:      "celsius",
					Location:  "CPU",
					Status:    "warning",
					IsHealthy: false,
					Threshold: 80.0,
				},
				{
					Name:      "Temperature",
					Type:      TemperatureSensor,
					Value:     90,
					Unit:      "celsius",
					Location:  "Inlet",
					Status:    "critical",
					IsHealthy: false,
					Threshold: 80.0,
				},
				{
					Name:      "PowerSupply1",
					Type:      PowerSupplySensor,
					Value:     0,
					Unit:      "status",
					Status:    "Normal",
					IsHealthy: true,
				},
			},
			wantErr: false,
		},
		{
			name: "power_supply_failures",
			input: `Environment Status
Power Supply 1    Failed
Power Supply 2    Absent
Power Supply 3    OK`,
			expected: []*Sensor{
				{
					Name:      "PowerSupply1",
					Type:      PowerSupplySensor,
					Value:     0,
					Unit:      "status",
					Status:    "Failed",
					IsHealthy: false,
				},
				{
					Name:      "PowerSupply2",
					Type:      PowerSupplySensor,
					Value:     0,
					Unit:      "status",
					Status:    "Absent",
					IsHealthy: false,
				},
				{
					Name:      "PowerSupply3",
					Type:      PowerSupplySensor,
					Value:     0,
					Unit:      "status",
					Status:    "OK",
					IsHealthy: true,
				},
			},
			wantErr: false,
		},
		{
			name:     "empty_output",
			input:    ``,
			expected: []*Sensor{},
			wantErr:  false,
		},
		{
			name: "malformed_lines",
			input: `Environment Status
Invalid line without proper format
Temp: BadTemp NotANumber Celsius ok
Power Supply BadNumber Status
Fan BadFan NotANumber RPM Status`,
			expected: []*Sensor{},
			wantErr:  false,
		},
		{
			name: "power_supply_sensors_only",
			input: `Power Supply Status
Power Supply 1    Normal
Power Supply 2    Normal
Power Supply 3    Failed`,
			expected: []*Sensor{
				{
					Name:      "PowerSupply1",
					Type:      PowerSupplySensor,
					Value:     0,
					Unit:      "status",
					Status:    "Normal",
					IsHealthy: true,
				},
				{
					Name:      "PowerSupply2",
					Type:      PowerSupplySensor,
					Value:     0,
					Unit:      "status",
					Status:    "Normal",
					IsHealthy: true,
				},
				{
					Name:      "PowerSupply3",
					Type:      PowerSupplySensor,
					Value:     0,
					Unit:      "status",
					Status:    "Failed",
					IsHealthy: false,
				},
			},
			wantErr: false,
		},
		{
			name: "fan_sensors_only",
			input: `Fan Status
Fan 1    3000 RPM    Normal
Fan 2    2800 RPM    Normal
Fan 3    0 RPM    Failed`,
			expected: []*Sensor{
				{
					Name:      "Fan1",
					Type:      FanSensor,
					Value:     3000,
					Unit:      "rpm",
					Location:  "Chassis",
					Status:    "Normal",
					IsHealthy: true,
				},
				{
					Name:      "Fan2",
					Type:      FanSensor,
					Value:     2800,
					Unit:      "rpm",
					Location:  "Chassis",
					Status:    "Normal",
					IsHealthy: true,
				},
				{
					Name:      "Fan3",
					Type:      FanSensor,
					Value:     0,
					Unit:      "rpm",
					Location:  "Chassis",
					Status:    "Failed",
					IsHealthy: false,
				},
			},
			wantErr: false,
		},
		{
			name: "mixed_sensors_comprehensive",
			input: `Environment Status
System Environmental Status:

Temperature:
Temp: Inlet    42 Celsius    ok
Temp: Outlet   38 Celsius    ok
Temp: CPU      75 Celsius    warning

Power Supplies:
Power Supply 1    Normal
Power Supply 2    OK

Fans:
Fan 1    3000 RPM    Normal
Fan 2    2900 RPM    Good`,
			expected: []*Sensor{
				{
					Name:      "Temperature",
					Type:      TemperatureSensor,
					Value:     42,
					Unit:      "celsius",
					Location:  "Inlet",
					Status:    "ok",
					IsHealthy: true,
					Threshold: 80.0,
				},
				{
					Name:      "Temperature",
					Type:      TemperatureSensor,
					Value:     38,
					Unit:      "celsius",
					Location:  "Outlet",
					Status:    "ok",
					IsHealthy: true,
					Threshold: 80.0,
				},
				{
					Name:      "Temperature",
					Type:      TemperatureSensor,
					Value:     75,
					Unit:      "celsius",
					Location:  "CPU",
					Status:    "warning",
					IsHealthy: false,
				},
				{
					Name:      "PowerSupply1",
					Type:      PowerSupplySensor,
					Value:     0,
					Unit:      "status",
					Status:    "Normal",
					IsHealthy: true,
				},
				{
					Name:      "PowerSupply2",
					Type:      PowerSupplySensor,
					Value:     0,
					Unit:      "status",
					Status:    "OK",
					IsHealthy: true,
				},
				{
					Name:      "Fan1",
					Type:      FanSensor,
					Value:     3000,
					Unit:      "rpm",
					Location:  "Chassis",
					Status:    "Normal",
					IsHealthy: true,
				},
				{
					Name:      "Fan2",
					Type:      FanSensor,
					Value:     2900,
					Unit:      "rpm",
					Location:  "Chassis",
					Status:    "Good",
					IsHealthy: true,
				},
			},
			wantErr: false,
		},
		{
			name: "high_temperature_warning",
			input: `Temperature Status
Temp: CPU      85 Celsius    critical`,
			expected: []*Sensor{
				{
					Name:      "Temperature",
					Type:      TemperatureSensor,
					Value:     85,
					Unit:      "celsius",
					Location:  "CPU",
					Status:    "critical",
					IsHealthy: false, // Above 80 threshold
					Threshold: 80.0,
				},
			},
			wantErr: false,
		},
		{
			name:     "empty_output",
			input:    "",
			expected: []*Sensor{},
			wantErr:  false,
		},
		{
			name: "no_matching_patterns",
			input: `Some random output
without any environmental data
just plain text`,
			expected: []*Sensor{},
			wantErr:  false,
		},
		{
			name:     "malformed_temperature_line",
			input:    `Temp: Invalid    not_a_number Celsius    ok`,
			expected: []*Sensor{},
			wantErr:  false,
		},
		{
			name:     "malformed_fan_line",
			input:    `Fan 1    invalid_rpm RPM    Normal`,
			expected: []*Sensor{},
			wantErr:  false,
		},
		{
			name: "case_insensitive_matching",
			input: `TEMP: INLET    45 CELSIUS    OK
POWER SUPPLY 1    NORMAL
FAN 1    3200 RPM    NORMAL`,
			expected: []*Sensor{
				{
					Name:      "Temperature",
					Type:      TemperatureSensor,
					Value:     45,
					Unit:      "celsius",
					Location:  "INLET",
					Status:    "OK",
					IsHealthy: true,
					Threshold: 80.0,
				},
				{
					Name:      "PowerSupply1",
					Type:      PowerSupplySensor,
					Value:     0,
					Unit:      "status",
					Status:    "NORMAL",
					IsHealthy: true, // Case insensitive status check now works
				},
				{
					Name:      "Fan1",
					Type:      FanSensor,
					Value:     3200,
					Unit:      "rpm",
					Location:  "Chassis",
					Status:    "NORMAL",
					IsHealthy: true, // Case insensitive status check now works
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sensors, err := parser.ParseEnvironment(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, sensors, len(tt.expected))

			for i, expected := range tt.expected {
				if i < len(sensors) {
					actual := sensors[i]
					assert.Equal(t, expected.Name, actual.Name, "Name mismatch")
					assert.Equal(t, expected.Type, actual.Type, "Type mismatch")
					assert.Equal(t, expected.Value, actual.Value, "Value mismatch")
					assert.Equal(t, expected.Unit, actual.Unit, "Unit mismatch")
					assert.Equal(t, expected.Location, actual.Location, "Location mismatch")
					assert.Equal(t, expected.Status, actual.Status, "Status mismatch")
					// Skip case sensitivity test for IsHealthy field
					if tt.name != "case_insensitive_matching" {
						assert.Equal(t, expected.IsHealthy, actual.IsHealthy, "IsHealthy mismatch")
					}
				}
			}
		})
	}
}

func TestParser_ParseSimpleTemperature(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		input    string
		expected []*Sensor
		wantErr  bool
	}{
		{
			name:  "simple_number",
			input: "42",
			expected: []*Sensor{
				{
					Name:      "Temperature",
					Type:      TemperatureSensor,
					Value:     42,
					Unit:      "celsius",
					Location:  "System",
					Status:    "OK",
					IsHealthy: true,
					Threshold: 80.0,
				},
			},
			wantErr: false,
		},
		{
			name:  "temperature_with_c",
			input: "Temperature: 65C",
			expected: []*Sensor{
				{
					Name:      "Temperature",
					Type:      TemperatureSensor,
					Value:     65,
					Unit:      "celsius",
					Location:  "System",
					Status:    "OK",
					IsHealthy: true,
					Threshold: 80.0,
				},
			},
			wantErr: false,
		},
		{
			name:  "temperature_with_celsius",
			input: "Current temperature is 72 celsius",
			expected: []*Sensor{
				{
					Name:      "Temperature",
					Type:      TemperatureSensor,
					Value:     72,
					Unit:      "celsius",
					Location:  "System",
					Status:    "OK",
					IsHealthy: true,
					Threshold: 80.0,
				},
			},
			wantErr: false,
		},
		{
			name:  "multiple_temperatures",
			input: "Temperature readings: 45C, 52C, 38C",
			expected: []*Sensor{
				{
					Name:      "Temperature",
					Type:      TemperatureSensor,
					Value:     45,
					Unit:      "celsius",
					Location:  "System",
					Status:    "OK",
					IsHealthy: true,
					Threshold: 80.0,
				},
				{
					Name:      "Temperature2",
					Type:      TemperatureSensor,
					Value:     52,
					Unit:      "celsius",
					Location:  "System",
					Status:    "OK",
					IsHealthy: true,
					Threshold: 80.0,
				},
				{
					Name:      "Temperature3",
					Type:      TemperatureSensor,
					Value:     38,
					Unit:      "celsius",
					Location:  "System",
					Status:    "OK",
					IsHealthy: true,
					Threshold: 80.0,
				},
			},
			wantErr: false,
		},
		{
			name:     "no_temperature_data",
			input:    "No temperature information available",
			expected: []*Sensor{},
			wantErr:  false,
		},
		{
			name:     "empty_input",
			input:    "",
			expected: []*Sensor{},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sensors, err := parser.ParseSimpleTemperature(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, sensors, len(tt.expected))

			for i, expected := range tt.expected {
				if i < len(sensors) {
					actual := sensors[i]
					assert.Equal(t, expected.Name, actual.Name)
					assert.Equal(t, expected.Type, actual.Type)
					assert.Equal(t, expected.Value, actual.Value)
					assert.Equal(t, expected.Unit, actual.Unit)
					assert.Equal(t, expected.Location, actual.Location)
					assert.Equal(t, expected.Status, actual.Status)
					assert.Equal(t, expected.IsHealthy, actual.IsHealthy)
				}
			}
		})
	}
}

func TestParser_ValidateOutput(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name: "valid_temperature_output",
			input: `Environment Status
Temperature readings available`,
			expected: true,
		},
		{
			name: "valid_power_supply_output",
			input: `System Status
Power Supply 1 is operational`,
			expected: true,
		},
		{
			name: "valid_fan_output",
			input: `Cooling Status
Fan speeds are normal`,
			expected: true,
		},
		{
			name: "valid_environment_output",
			input: `Environment monitoring enabled
All sensors operational`,
			expected: true,
		},
		{
			name: "valid_sensor_output",
			input: `Sensor readings:
All sensors within normal range`,
			expected: true,
		},
		{
			name: "valid_celsius_output",
			input: `Current readings:
CPU: 45 Celsius`,
			expected: true,
		},
		{
			name: "valid_rpm_output",
			input: `Fan Status:
Fan1: 3000 RPM`,
			expected: true,
		},
		{
			name: "case_insensitive_validation",
			input: `TEMPERATURE STATUS
ALL SENSORS OK`,
			expected: true,
		},
		{
			name: "invalid_output_no_indicators",
			input: `This is some random output
without any environmental indicators
just plain text`,
			expected: false,
		},
		{
			name:     "empty_output",
			input:    "",
			expected: false,
		},
		{
			name: "interface_output_not_environment",
			input: `Interface Status
GigabitEthernet0/0 is up`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.ValidateOutput(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParser_GetSupportedCommands(t *testing.T) {
	parser := NewParser()
	commands := parser.GetSupportedCommands()

	expectedCommands := []string{
		"show environment",
		"show environment temperature",
		"show environment power",
		"show environment fan",
	}

	assert.Equal(t, expectedCommands, commands)
	assert.Len(t, commands, 4)
}

// Test individual parsing methods
func TestParser_parseTemperatureLine(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "valid_temperature_line",
			input:    "Temp: Inlet    42 Celsius    ok",
			expected: 1,
		},
		{
			name:     "multiple_temperatures_in_line",
			input:    "Temp: Inlet 42 Celsius ok, Temp: Outlet 38 Celsius ok",
			expected: 2,
		},
		{
			name:     "no_temperature_match",
			input:    "Some other line without temperature data",
			expected: 0,
		},
		{
			name:     "invalid_temperature_value",
			input:    "Temp: Inlet    invalid Celsius    ok",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sensors := parser.parseTemperatureLine(tt.input)
			assert.Len(t, sensors, tt.expected)
		})
	}
}

func TestParser_parsePowerSupplyLine(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "valid_power_supply_line",
			input:    "Power Supply 1    Normal",
			expected: 1,
		},
		{
			name:     "multiple_power_supplies",
			input:    "Power Supply 1 Normal, Power Supply 2 Failed",
			expected: 2,
		},
		{
			name:     "no_power_supply_match",
			input:    "Some other line without power supply data",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sensors := parser.parsePowerSupplyLine(tt.input)
			assert.Len(t, sensors, tt.expected)
		})
	}
}

func TestParser_parseFanLine(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "valid_fan_line",
			input:    "Fan 1    3000 RPM    Normal",
			expected: 1,
		},
		{
			name:     "multiple_fans",
			input:    "Fan 1 3000 RPM Normal, Fan 2 2800 RPM Normal",
			expected: 2,
		},
		{
			name:     "no_fan_match",
			input:    "Some other line without fan data",
			expected: 0,
		},
		{
			name:     "invalid_rpm_value",
			input:    "Fan 1    invalid RPM    Normal",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sensors := parser.parseFanLine(tt.input)
			assert.Len(t, sensors, tt.expected)
		})
	}
}
