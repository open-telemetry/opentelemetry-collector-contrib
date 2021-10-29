// Copyright 2021 Google LLC
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

package opentelemetry_collector_contrib

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRequiredDepsOnly(t *testing.T) {
	changeFilesArray := getChangedFilesArray("git diff --name-only HEAD^..HEAD", t)
	fileChangePathSet := getChangedPathSet(changeFilesArray, t)
	moduleChangePathSet := getChangedModulesSet(fileChangePathSet, t)
	pathSetOfModules := getAllModules(t)
	modulesToTest := getAllModulesToTest(pathSetOfModules, moduleChangePathSet, t)
	//test modules
	for s := range modulesToTest {
		out, err := exec.Command("bash", "-c", "cd " + s + "; go test ./... -v").Output()
		t.Log(s + " test output " + string(out))
		if err != nil {

			t.Fatal(err)
		}
	}
}

func TestRequiredFilesArray(t *testing.T) {
	changeFilesArray := getChangedFilesArray("git diff --name-only 9a8054c4c9d05e8fbac92acc0cbd0bd7511d5a7a^..a75347590dfa92c153c1d4f815cb599e6afe9f45", t)
	changeFilesArrayExpected := []string{".github/CODEOWNERS",
		"extension/bearertokenauthextension/README.md",
		"extension/bearertokenauthextension/bearertokenauth.go",
		"extension/bearertokenauthextension/bearertokenauth_test.go",
		""}
	assert.Equal(t, changeFilesArrayExpected, changeFilesArray)
}

func TestFileChangePathSet(t *testing.T) {
	changeFilesArray := getChangedFilesArray("git diff --name-only 9a8054c4c9d05e8fbac92acc0cbd0bd7511d5a7a^..a75347590dfa92c153c1d4f815cb599e6afe9f45", t)
	fileChangePathSet := getChangedPathSet(changeFilesArray, t)
	fileChangePathSetExpected := map[string]bool{".github/":true, "extension/bearertokenauthextension/":true}
	assert.Equal(t, fileChangePathSetExpected, fileChangePathSet)
}

func TestModuleChangePathSet(t *testing.T) {
	changeFilesArray := getChangedFilesArray("git diff --name-only 9a8054c4c9d05e8fbac92acc0cbd0bd7511d5a7a^..a75347590dfa92c153c1d4f815cb599e6afe9f45", t)
	fileChangePathSet := getChangedPathSet(changeFilesArray, t)
	moduleChangePathSet := getChangedModulesSet(fileChangePathSet, t)
	moduleChangePathSetExpected := map[string]bool{"extension/bearertokenauthextension/":true}
	assert.Equal(t, moduleChangePathSetExpected, moduleChangePathSet)
}

func TestPathSetOfModules(t *testing.T) {
	modulesPathSet := getAllModules(t)
	t.Log(modulesPathSet)
	modulesPathSetExpected := map[string]bool{"../opentelemetry-collector-contrib/":true, "../opentelemetry-collector-contrib/cmd/mdatagen/":true, "../opentelemetry-collector-contrib/examples/demo/client/":true, "../opentelemetry-collector-contrib/examples/demo/server/":true, "../opentelemetry-collector-contrib/exporter/alibabacloudlogserviceexporter/":true, "../opentelemetry-collector-contrib/exporter/awscloudwatchlogsexporter/":true, "../opentelemetry-collector-contrib/exporter/awsemfexporter/":true, "../opentelemetry-collector-contrib/exporter/awskinesisexporter/":true, "../opentelemetry-collector-contrib/exporter/awsprometheusremotewriteexporter/":true, "../opentelemetry-collector-contrib/exporter/awsxrayexporter/":true, "../opentelemetry-collector-contrib/exporter/azuremonitorexporter/":true, "../opentelemetry-collector-contrib/exporter/carbonexporter/":true, "../opentelemetry-collector-contrib/exporter/datadogexporter/":true, "../opentelemetry-collector-contrib/exporter/dynatraceexporter/":true, "../opentelemetry-collector-contrib/exporter/elasticexporter/":true, "../opentelemetry-collector-contrib/exporter/elasticsearchexporter/":true, "../opentelemetry-collector-contrib/exporter/f5cloudexporter/":true, "../opentelemetry-collector-contrib/exporter/fileexporter/":true, "../opentelemetry-collector-contrib/exporter/googlecloudexporter/":true, "../opentelemetry-collector-contrib/exporter/googlecloudpubsubexporter/":true, "../opentelemetry-collector-contrib/exporter/honeycombexporter/":true, "../opentelemetry-collector-contrib/exporter/humioexporter/":true, "../opentelemetry-collector-contrib/exporter/influxdbexporter/":true, "../opentelemetry-collector-contrib/exporter/jaegerexporter/":true, "../opentelemetry-collector-contrib/exporter/kafkaexporter/":true, "../opentelemetry-collector-contrib/exporter/loadbalancingexporter/":true, "../opentelemetry-collector-contrib/exporter/logzioexporter/":true, "../opentelemetry-collector-contrib/exporter/lokiexporter/":true, "../opentelemetry-collector-contrib/exporter/newrelicexporter/":true, "../opentelemetry-collector-contrib/exporter/observiqexporter/":true, "../opentelemetry-collector-contrib/exporter/opencensusexporter/":true, "../opentelemetry-collector-contrib/exporter/prometheusexporter/":true, "../opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter/":true, "../opentelemetry-collector-contrib/exporter/sapmexporter/":true, "../opentelemetry-collector-contrib/exporter/sentryexporter/":true, "../opentelemetry-collector-contrib/exporter/signalfxexporter/":true, "../opentelemetry-collector-contrib/exporter/skywalkingexporter/":true, "../opentelemetry-collector-contrib/exporter/splunkhecexporter/":true, "../opentelemetry-collector-contrib/exporter/stackdriverexporter/":true, "../opentelemetry-collector-contrib/exporter/sumologicexporter/":true, "../opentelemetry-collector-contrib/exporter/tanzuobservabilityexporter/":true, "../opentelemetry-collector-contrib/exporter/zipkinexporter/":true, "../opentelemetry-collector-contrib/extension/awsproxy/":true, "../opentelemetry-collector-contrib/extension/bearertokenauthextension/":true, "../opentelemetry-collector-contrib/extension/fluentbitextension/":true, "../opentelemetry-collector-contrib/extension/healthcheckextension/":true, "../opentelemetry-collector-contrib/extension/httpforwarder/":true, "../opentelemetry-collector-contrib/extension/oauth2clientauthextension/":true, "../opentelemetry-collector-contrib/extension/observer/":true, "../opentelemetry-collector-contrib/extension/observer/dockerobserver/":true, "../opentelemetry-collector-contrib/extension/observer/ecsobserver/":true, "../opentelemetry-collector-contrib/extension/observer/hostobserver/":true, "../opentelemetry-collector-contrib/extension/observer/k8sobserver/":true, "../opentelemetry-collector-contrib/extension/oidcauthextension/":true, "../opentelemetry-collector-contrib/extension/pprofextension/":true, "../opentelemetry-collector-contrib/extension/storage/":true, "../opentelemetry-collector-contrib/internal/aws/awsutil/":true, "../opentelemetry-collector-contrib/internal/aws/containerinsight/":true, "../opentelemetry-collector-contrib/internal/aws/k8s/":true, "../opentelemetry-collector-contrib/internal/aws/metrics/":true, "../opentelemetry-collector-contrib/internal/aws/proxy/":true, "../opentelemetry-collector-contrib/internal/aws/xray/":true, "../opentelemetry-collector-contrib/internal/aws/xray/testdata/sampleapp/":true, "../opentelemetry-collector-contrib/internal/aws/xray/testdata/sampleserver/":true, "../opentelemetry-collector-contrib/internal/common/":true, "../opentelemetry-collector-contrib/internal/coreinternal/":true, "../opentelemetry-collector-contrib/internal/docker/":true, "../opentelemetry-collector-contrib/internal/k8sconfig/":true, "../opentelemetry-collector-contrib/internal/kubelet/":true, "../opentelemetry-collector-contrib/internal/scrapertest/":true, "../opentelemetry-collector-contrib/internal/sharedcomponent/":true, "../opentelemetry-collector-contrib/internal/splunk/":true, "../opentelemetry-collector-contrib/internal/stanza/":true, "../opentelemetry-collector-contrib/internal/tools/":true, "../opentelemetry-collector-contrib/pkg/batchperresourceattr/":true, "../opentelemetry-collector-contrib/pkg/batchpersignal/":true, "../opentelemetry-collector-contrib/pkg/experimentalmetricmetadata/":true, "../opentelemetry-collector-contrib/pkg/resourcetotelemetry/":true, "../opentelemetry-collector-contrib/pkg/translator/jaeger/":true, "../opentelemetry-collector-contrib/pkg/translator/opencensus/":true, "../opentelemetry-collector-contrib/pkg/translator/zipkin/":true, "../opentelemetry-collector-contrib/processor/attributesprocessor/":true, "../opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor/":true, "../opentelemetry-collector-contrib/processor/deltatorateprocessor/":true, "../opentelemetry-collector-contrib/processor/filterprocessor/":true, "../opentelemetry-collector-contrib/processor/groupbyattrsprocessor/":true, "../opentelemetry-collector-contrib/processor/groupbytraceprocessor/":true, "../opentelemetry-collector-contrib/processor/k8sattributesprocessor/":true, "../opentelemetry-collector-contrib/processor/metricsgenerationprocessor/":true, "../opentelemetry-collector-contrib/processor/metricstransformprocessor/":true, "../opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor/":true, "../opentelemetry-collector-contrib/processor/resourcedetectionprocessor/":true, "../opentelemetry-collector-contrib/processor/resourceprocessor/":true, "../opentelemetry-collector-contrib/processor/routingprocessor/":true, "../opentelemetry-collector-contrib/processor/spanmetricsprocessor/":true, "../opentelemetry-collector-contrib/processor/spanprocessor/":true, "../opentelemetry-collector-contrib/processor/tailsamplingprocessor/":true, "../opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/":true, "../opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver/":true, "../opentelemetry-collector-contrib/receiver/awsxrayreceiver/":true, "../opentelemetry-collector-contrib/receiver/carbonreceiver/":true, "../opentelemetry-collector-contrib/receiver/cloudfoundryreceiver/":true, "../opentelemetry-collector-contrib/receiver/collectdreceiver/":true, "../opentelemetry-collector-contrib/receiver/dockerstatsreceiver/":true, "../opentelemetry-collector-contrib/receiver/dotnetdiagnosticsreceiver/":true, "../opentelemetry-collector-contrib/receiver/filelogreceiver/":true, "../opentelemetry-collector-contrib/receiver/fluentforwardreceiver/":true, "../opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver/":true, "../opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/":true, "../opentelemetry-collector-contrib/receiver/hostmetricsreceiver/":true, "../opentelemetry-collector-contrib/receiver/httpdreceiver/":true, "../opentelemetry-collector-contrib/receiver/influxdbreceiver/":true, "../opentelemetry-collector-contrib/receiver/jaegerreceiver/":true, "../opentelemetry-collector-contrib/receiver/jmxreceiver/":true, "../opentelemetry-collector-contrib/receiver/journaldreceiver/":true, "../opentelemetry-collector-contrib/receiver/k8sclusterreceiver/":true, "../opentelemetry-collector-contrib/receiver/k8seventsreceiver/":true, "../opentelemetry-collector-contrib/receiver/kafkametricsreceiver/":true, "../opentelemetry-collector-contrib/receiver/kafkareceiver/":true, "../opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/":true, "../opentelemetry-collector-contrib/receiver/memcachedreceiver/":true, "../opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/":true, "../opentelemetry-collector-contrib/receiver/mysqlreceiver/":true, "../opentelemetry-collector-contrib/receiver/nginxreceiver/":true, "../opentelemetry-collector-contrib/receiver/opencensusreceiver/":true, "../opentelemetry-collector-contrib/receiver/podmanreceiver/":true, "../opentelemetry-collector-contrib/receiver/prometheusexecreceiver/":true, "../opentelemetry-collector-contrib/receiver/prometheusreceiver/":true, "../opentelemetry-collector-contrib/receiver/receivercreator/":true, "../opentelemetry-collector-contrib/receiver/redisreceiver/":true, "../opentelemetry-collector-contrib/receiver/sapmreceiver/":true, "../opentelemetry-collector-contrib/receiver/signalfxreceiver/":true, "../opentelemetry-collector-contrib/receiver/simpleprometheusreceiver/":true, "../opentelemetry-collector-contrib/receiver/simpleprometheusreceiver/examples/federation/prom-counter/":true, "../opentelemetry-collector-contrib/receiver/splunkhecreceiver/":true, "../opentelemetry-collector-contrib/receiver/statsdreceiver/":true, "../opentelemetry-collector-contrib/receiver/syslogreceiver/":true, "../opentelemetry-collector-contrib/receiver/tcplogreceiver/":true, "../opentelemetry-collector-contrib/receiver/udplogreceiver/":true, "../opentelemetry-collector-contrib/receiver/wavefrontreceiver/":true, "../opentelemetry-collector-contrib/receiver/windowsperfcountersreceiver/":true, "../opentelemetry-collector-contrib/receiver/zipkinreceiver/":true, "../opentelemetry-collector-contrib/receiver/zookeeperreceiver/":true, "../opentelemetry-collector-contrib/testbed/":true, "../opentelemetry-collector-contrib/testbed/mockdatareceivers/mockawsxrayreceiver/":true, "../opentelemetry-collector-contrib/tracegen/":true}
	assert.Equal(t, modulesPathSetExpected, modulesPathSet)
}

func TestModulesToTest(t *testing.T) {
	changeFilesArray := getChangedFilesArray("git diff --name-only 9a8054c4c9d05e8fbac92acc0cbd0bd7511d5a7a^..a75347590dfa92c153c1d4f815cb599e6afe9f45", t)
	fileChangePathSet := getChangedPathSet(changeFilesArray, t)
	moduleChangePathSet := getChangedModulesSet(fileChangePathSet, t)
	modulesPathSet := getAllModules(t)
	modulesToTest := getAllModulesToTest(modulesPathSet, moduleChangePathSet, t)
	modulesToTestExpected := map[string]bool{"../opentelemetry-collector-contrib/extension/bearertokenauthextension/":true}
	assert.Equal(t, modulesToTestExpected, modulesToTest)
}


func getChangedFilesArray(execCommand string, t *testing.T) []string {
	//get all files that changed in last commit
	out, _ := exec.Command("bash", "-c", execCommand).Output()
	fileChanged := string(out)
	t.Log("Files Changed " + fileChanged)
	filesChangedArray := strings.Split(fileChanged, "\n")
	return filesChangedArray
}

func getChangedPathSet(filesChangedArray []string, t *testing.T) map[string]bool {
	fileChangePathSet := make(map[string]bool)
	//get the path of files that changed
	for i := range filesChangedArray {
		dir, _ := filepath.Split(filesChangedArray[i])
		if len(dir) == 0 {
			continue
		}
		fileChangePathSet[dir] = true;
	}
	for s := range fileChangePathSet {
		t.Log("Path Changed " + s)
	}
	return fileChangePathSet
}

func getChangedModulesSet(fileChangePathSet map[string]bool, t *testing.T) map[string]bool {
	//get path of all modules that changed
	moduleChangePathSet := make(map[string]bool)
	for s := range fileChangePathSet {
		path := recursiveFindClosestModule(s)
		//this happens when a file is changed outside of a module such as github workflows
		if path =="." {
			continue
		}
		moduleChangePathSet[path] = true
	}
	for s := range moduleChangePathSet {
		t.Log("Module Path Changed " + s)
	}
	return moduleChangePathSet
}

func getAllModules(t *testing.T) map[string]bool {
	//get all modules
	pathSetOfModules := make(map[string]bool)
	_ = filepath.Walk("..", func(path string, info os.FileInfo, err error) error {
		if info.Name() == "go.mod" {
			dir, _ := filepath.Split(path)
			pathSetOfModules[dir] = true
		}
		return nil
	})
	for s := range pathSetOfModules {
		t.Log("Modules path " + s)
	}
	if pathSetOfModules["../opentelemetry-collector-contrib/cmd/configschema/"] {
		delete(pathSetOfModules, "../opentelemetry-collector-contrib/cmd/configschema/")
	}
	return pathSetOfModules
}

func getAllModulesToTest(pathSetOfModules map[string]bool, moduleChangePathSet map[string]bool, t *testing.T) map[string]bool {
	//get all modules to test
	modulesToTest := make(map[string]bool)
	for modPath := range pathSetOfModules {
		for moduleChangePath := range moduleChangePathSet {
			if modulesToTest[modPath] {
				break
			}
			out, _ := exec.Command("bash", "-c",
				"cd " + modPath + "; go mod graph | grep " + moduleChangePath).
				Output()
			if len(string(out)) > 0 || strings.Contains(modPath, moduleChangePath) {
				modulesToTest[modPath] = true
			}
		}
	}
	//filter out this directory
	delete(modulesToTest, "../opentelemetry-collector-contrib/")
	for s := range modulesToTest {
		t.Log("Test module " + s)
	}
	return modulesToTest
}

func recursiveFindClosestModule(path string) string {
	if _, err := os.Stat(path + "/go.mod");
		!errors.Is(err, os.ErrNotExist) {
		return path
	}
	path = filepath.Clean(filepath.Join(path, ".."))
	return recursiveFindClosestModule(path)
}