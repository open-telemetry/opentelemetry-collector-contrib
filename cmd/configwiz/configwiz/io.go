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

package configwiz

import (
	"strings"
)

const defaultFileName = "compose.yaml"

type Clio struct {
	Write func(s string)
	Read  func(defaultVal string) string
}

func (io Clio) newIndentingPrinter(lvl int) (p indentingPrinter) {
	p = indentingPrinter{level: lvl, write: io.Write}
	return
}

// promptFileName prompts the user to input a fileName, and returns the value that they chose.
// defaults to out.yaml
func promptFileName(io Clio) string {
	pr := io.newIndentingPrinter(0)
	pr.println("Name of file (default out.yaml):")
	pr.print("> ")
	fileName := io.Read("")
	if fileName == "" {
		fileName = defaultFileName
	}
	if !strings.HasSuffix(fileName, ".yaml") {
		fileName += ".yaml"
	}
	return fileName
}
