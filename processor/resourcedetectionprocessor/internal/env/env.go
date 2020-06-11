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

// Package env provides a detector that loads resource information from
// the OTEL_RESOURCE environment variable. A list of labels of the form
// `<key1>=<value1>,<key2>=<value2>,...` is accepted. Domain names and
// paths are accepted as label keys.
package env

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"

	"go.opentelemetry.io/collector/consumer/pdata"
)

const TypeStr = "env"

// Environment variable used by "env" to decode a resource.
const envVar = "OTEL_RESOURCE"

type Detector struct{}

func (d *Detector) Detect(context.Context) (pdata.Resource, error) {
	res := pdata.NewResource()
	res.InitEmpty()

	labels := strings.TrimSpace(os.Getenv(envVar))
	if labels == "" {
		return res, nil
	}

	err := initializeAttributeMap(res.Attributes(), labels)
	if err != nil {
		res.Attributes().InitEmptyWithCapacity(0)
		return res, err
	}

	return res, nil
}

var labelRegex = regexp.MustCompile(`^\s*([[:ascii:]]{1,256}?)\s*=\s*([[:ascii:]]{0,256}?)\s*(?:,|$)`)

func initializeAttributeMap(am pdata.AttributeMap, s string) error {
	for len(s) > 0 {
		match := labelRegex.FindStringSubmatch(s)
		if len(match) == 0 {
			return fmt.Errorf("invalid resource format, remainder: %s", s)
		}
		v := match[2]
		if v == "" {
			v = match[3]
		} else {
			var err error
			if v, err = url.QueryUnescape(v); err != nil {
				return fmt.Errorf("invalid resource format, remainder: %s, err: %s", s, err)
			}
		}
		am.InsertString(match[1], v)

		s = s[len(match[0]):]
	}

	return nil
}
