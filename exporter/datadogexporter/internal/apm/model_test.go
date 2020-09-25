/*
 * Unless explicitly stated otherwise all files in this repository are licensed
 * under the Apache License Version 2.0.
 *
 * This product includes software developed at Datadog (https://www.datadoghq.com/).
 * Copyright 2020 Datadog, Inc.
 */
package datadogexporter

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/DataDog/datadog-agent/pkg/trace/obfuscate"
	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"

	"io/ioutil"
)

type integrationTestData struct {
	Comment string      `json:"comment"`
	Tags    string      `json:"tags"`
	Trace   interface{} `json:"trace"`
}

func CompareSnapshot(t *testing.T, inputFile, snapshotFile string, updateSnapshots bool) {

	// Spew is used to serialize snapshots, for
	sc := spew.NewDefaultConfig()
	sc.DisablePointerAddresses = true
	sc.SortKeys = true
	sc.DisableCapacities = true
	sc.DisablePointerMethods = true
	sc.DisableMethods = true
	sc.SpewKeys = true

	file, err := os.Open(inputFile)
	assert.NoError(t, err)
	defer file.Close()

	contents, err := ioutil.ReadAll(file)

	assert.NoError(t, err, "Couldn't read contents of test file")

	var td integrationTestData
	err = json.Unmarshal(contents, &td)
	assert.NoError(t, err, "Couldn't parse contents of test file")

	tc, _ := json.Marshal(td.Trace)
	traceContents := string(tc[:])

	obfuscator := obfuscate.NewObfuscator(&obfuscate.Config{
		ES: obfuscate.JSONSettings{
			Enabled: true,
		},
		Mongo: obfuscate.JSONSettings{
			Enabled: true,
		},
		RemoveQueryString: true,
		RemovePathDigits:  true,
		RemoveStackTraces: true,
		Redis:             true,
		Memcached:         true,
	})

	payload, err := ProcessTrace(traceContents, obfuscator, td.Tags)
	AddTagsToTracePayloads(payload, td.Tags)
	assert.NoError(t, err, "Couldn't parse trace")

	output := sc.Sdump(payload)

	if updateSnapshots {
		err = ioutil.WriteFile(snapshotFile, []byte(output), 0644)
		assert.NoError(t, err)
		fmt.Printf("Updated Snapshot %s\n", snapshotFile)
	} else {
		snapshot, err := ioutil.ReadFile(snapshotFile)
		assert.NoError(t, err, "Missing snapshot file for test")
		expected := string(snapshot)
		assert.Equal(t, expected, output, fmt.Sprintf("Snapshot's didn't match for %s. To update run `$UPDATE_SNAPSHOTS=true go test ./...`", inputFile))
	}
}

func TestSnapshotsMatch(t *testing.T) {
	files, err := ioutil.ReadDir("testdata")
	assert.NoError(t, err)
	us := os.Getenv("UPDATE_SNAPSHOTS")
	updateSnapshots := strings.ToLower(us) == "true"

	for _, f := range files {

		if !strings.HasSuffix(f.Name(), ".json") {
			continue
		}
		inputFile := fmt.Sprintf("testdata/%s", f.Name())

		snapshotFile := fmt.Sprintf("%s~snapshot", inputFile)
		CompareSnapshot(t, inputFile, snapshotFile, updateSnapshots)
	}
}
