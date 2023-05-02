// Copyright The OpenTelemetry Authors
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

//go:build !windows
// +build !windows

package subprocessmanager // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusexecreceiver/subprocessmanager"

import (
	"os/exec"

	"github.com/kballard/go-shellquote"
)

// Non-Windows version of exec.Command(...)
// Compiles on all but Windows
func ExecCommand(commandLine string) (*exec.Cmd, error) {

	var args, err = shellquote.Split(commandLine)
	if err != nil {
		return nil, err
	}

	return exec.Command(args[0], args[1:]...), nil // #nosec

}
