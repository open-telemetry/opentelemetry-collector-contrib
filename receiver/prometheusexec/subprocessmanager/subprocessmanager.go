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

package subprocessmanager

import "fmt"

type process struct {
	command        string
	port           int
	scrapeInterval string
	env            []string
}

// ManageProcesses will call the necessary helper functions to start and handle the processes
func ManageProcesses(processes []process) {
	for _, proc := range processes {
		fmt.Printf("%+v", proc)
	}
}
