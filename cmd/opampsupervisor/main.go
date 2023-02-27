// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"os"
	"os/signal"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor"
	"go.uber.org/zap"
)

func main() {
	configFlag := flag.String("config", "", "Path to a supervisor configuration file")
	flag.Parse()

	logger, _ := zap.NewDevelopment()

	supervisor, err := supervisor.NewSupervisor(logger, *configFlag)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(-1)
		return
	}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	<-interrupt
	supervisor.Shutdown()
}
