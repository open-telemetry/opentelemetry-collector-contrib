// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tests

import (
	"log"
	"os"

	"github.com/spf13/viper"
)

func getSplunkUser() string {
	return os.Getenv("CI_SPLUNK_USERNAME")
}

func getSplunkPassword() string {
	return os.Getenv("CI_SPLUNK_PASSWORD")
}

func getSplunkHost() string {
	return os.Getenv("CI_SPLUNK_HOST")
}

func getSplunkPort() string {
	return os.Getenv("CI_SPLUNK_PORT")
}

func viperConfigVariable(key string) string {
	viper.SetConfigName("data")
	viper.AddConfigPath("./config")

	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("Error while reading config file %s", err)
	}

	value, ok := viper.Get(key).(string)
	if !ok {
		log.Fatalf("Invalid type assertion")
	}
	return value
}
