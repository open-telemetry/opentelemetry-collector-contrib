// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"log"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/configschema/otto/otto"
)

const defaultAddress = ":8888"

func main() {
	logger := log.Default()
	addrPtr := flag.String("port", defaultAddress, "the address (host:port) that otto will listen on")
	collectorDirPtr := flag.String("collector_src", ".", "the root directory of a locally checked-out collector repo")
	flag.Parse()
	otto.Server(logger, *addrPtr, *collectorDirPtr)
}
