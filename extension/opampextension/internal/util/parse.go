// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package util // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampextension/internal/util"

import "strings"

// ParseDarwinDescription parses out the OS name and version for darwin machines.
// 'input' should be the string representation from running the command 'sw_vers'.
// An example of this command's desired output is below:
//
// ProductName:		    macOS
// ProductVersion:		15.0.1
// BuildVersion:		24A348
func ParseDarwinDescription(input string) string {
	var productName string
	var productVersion string
	for _, l := range strings.Split(input, "\n") {
		line := strings.TrimSpace(l)
		if raw, ok := strings.CutPrefix(line, "ProductName:"); ok {
			productName = strings.TrimSpace(raw)
		} else if raw, ok = strings.CutPrefix(line, "ProductVersion:"); ok {
			productVersion = strings.TrimSpace(raw)
		}
	}
	if productName != "" && productVersion != "" {
		return productName + " " + productVersion
	}
	return ""
}

// ParseLinuxDescription parses out the OS name and version for linux machines.
// 'input' should be the string representation from running the command 'lsb_release -d'.
// An example of this command's desired output is below:
//
// Description:	    Ubuntu 20.04.6 LTS
func ParseLinuxDescription(input string) string {
	if raw, ok := strings.CutPrefix(strings.TrimSpace(input), "Description:"); ok {
		return strings.TrimSpace(raw)
	}
	return ""
}

// ParseLinuxDescription parses out the OS name and version for windows machines.
// 'input' should be the string representation from running the command 'cmd /c ver'.
// An example of this command's desired output is below:
//
// Microsoft Windows [Version 10.0.20348.2700]
func ParseWindowsDescription(input string) string {
	return strings.TrimSpace(input)
}
