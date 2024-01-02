// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package integrationtestutils // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter/internal/integrationtestutils"

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

var configFilePth = "./testdata/integration_tests_config.yaml"

type IntegrationTestsConfig struct {
	Host           string `yaml:"HOST"`
	User           string `yaml:"USER"`
	Password       string `yaml:"PASSWORD"`
	UIPort         string `yaml:"UI_PORT"`
	HecPort        string `yaml:"HEC_PORT"`
	ManagementPort string `yaml:"MANAGEMENT_PORT"`
	EventIndex     string `yaml:"EVENT_INDEX"`
	MetricIndex    string `yaml:"METRIC_INDEX"`
	TraceIndex     string `yaml:"TRACE_INDEX"`
	HecToken       string `yaml:"HEC_TOKEN"`
	SplunkImage    string `yaml:"SPLUNK_IMAGE"`
}

func GetConfigVariable(key string) string {
	// Read YAML file
	fileData, err := os.ReadFile(configFilePth)
	if err != nil {
		fmt.Println("Error reading file:", err)
	}

	var config IntegrationTestsConfig
	err = yaml.Unmarshal(fileData, &config)
	if err != nil {
		fmt.Println("Error decoding YAML:", err)
	}

	switch key {
	case "HOST":
		return config.Host
	case "USER":
		return config.User
	case "PASSWORD":
		return config.Password
	case "UI_PORT":
		return config.UIPort
	case "HEC_PORT":
		return config.HecPort
	case "MANAGEMENT_PORT":
		return config.ManagementPort
	case "EVENT_INDEX":
		return config.EventIndex
	case "METRIC_INDEX":
		return config.MetricIndex
	case "TRACE_INDEX":
		return config.TraceIndex
	case "HEC_TOKEN":
		return config.HecToken
	case "SPLUNK_IMAGE":
		return config.SplunkImage
	default:
		fmt.Println("Invalid field")
		return "None"
	}
}

func SetConfigVariable(key string, value string) {
	// Read YAML file
	fileData, err := os.ReadFile(configFilePth)
	if err != nil {
		fmt.Println("Error reading file:", err)
	}

	var config IntegrationTestsConfig
	err = yaml.Unmarshal(fileData, &config)
	if err != nil {
		fmt.Printf("Error unmarshaling YAML: %v", err)
	}

	switch key {
	case "HOST":
		config.Host = value
	case "UI_PORT":
		config.UIPort = value
	case "HEC_PORT":
		config.HecPort = value
	case "MANAGEMENT_PORT":
		config.ManagementPort = value
	case "EVENT_INDEX":
	default:
		fmt.Println("Invalid field")
	}
	// Marshal updated Config into YAML
	newData, err := yaml.Marshal(&config)
	if err != nil {
		fmt.Printf("Error marshaling YAML: %v", err)
		return
	}

	// Write yaml file
	err = os.WriteFile(configFilePth, newData, os.ModePerm)
	if err != nil {
		fmt.Printf("Error writing file: %v", err)
		return
	}

	fmt.Println("Host value updated successfully!")

}
