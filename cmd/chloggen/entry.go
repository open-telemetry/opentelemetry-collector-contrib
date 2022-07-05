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

package main

import (
	"fmt"
	"strings"
)

type Entry struct {
	ChangeType string `yaml:"change_type"`
	Component  string `yaml:"component"`
	Note       string `yaml:"note"`
	Issues     []int  `yaml:"issues"`
	SubText    string `yaml:"subtext"`
}

var changeTypes = []string{
	breaking,
	deprecation,
	newComponent,
	enhancement,
	bugFix,
}

func (e Entry) Validate() error {
	var validType bool
	for _, ct := range changeTypes {
		if e.ChangeType == ct {
			validType = true
			break
		}
	}
	if !validType {
		return fmt.Errorf("'%s' is not a valid 'change_type'. Specify one of %v", e.ChangeType, changeTypes)
	}

	if e.Component == "" {
		return fmt.Errorf("specify a 'component'")
	}

	if e.Note == "" {
		return fmt.Errorf("specify a 'note'")
	}

	if len(e.Issues) == 0 {
		return fmt.Errorf("specify one or more issues #'s")
	}

	return nil
}

func (e Entry) String() string {
	issueStrs := make([]string, 0, len(e.Issues))
	for _, issue := range e.Issues {
		issueStrs = append(issueStrs, fmt.Sprintf("#%d", issue))
	}
	issueStr := strings.Join(issueStrs, ", ")

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("- `%s`: %s (%s)", e.Component, e.Note, issueStr))
	if e.SubText != "" {
		sb.WriteString("\n  ")
		lines := strings.Split(strings.ReplaceAll(e.SubText, "\r\n", "\n"), "\n")
		sb.WriteString(strings.Join(lines, "\n  "))
	}
	return sb.String()
}
