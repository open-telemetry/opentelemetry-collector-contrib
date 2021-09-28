// Copyright  OpenTelemetry Authors
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

package mysqlreceiver

import (
	"bufio"
	"os"
	"path"
	"strings"
)

var _ client = (*fakeClient)(nil)

type fakeClient struct {
}

func readFile(fname string) (map[string]string, error) {
	var stats = map[string]string{}
	file, err := os.Open(path.Join("testdata", "scraper", fname+".txt"))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		text := strings.Split(scanner.Text(), "\t")
		stats[text[0]] = text[1]
	}
	return stats, nil
}

func (c *fakeClient) getGlobalStats() (map[string]string, error) {
	return readFile("global_stats")
}

func (c *fakeClient) getInnodbStats() (map[string]string, error) {
	return readFile("innodb_stats")
}

func (c *fakeClient) Close() error {
	return nil
}
