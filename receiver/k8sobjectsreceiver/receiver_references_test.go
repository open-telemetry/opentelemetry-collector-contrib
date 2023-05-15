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

package k8sobjectsreceiver

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver/receivertest"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
)

func TestReferences(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Receiver References tests")
}

var _ = Context("Lumigo controller", func() {

	It("Send all needed Pod references", func() {
		mockClient := newMockDynamicClient()

		rCfg := createDefaultConfig().(*Config)
		rCfg.makeDynamicClient = mockClient.getMockDynamicClient
		rCfg.makeDiscoveryClient = getMockDiscoveryClient

		rCfg.Objects = []*K8sObjectsConfig{
			{
				Name:       "pods",
				Mode:       WatchMode,
				Namespaces: []string{"default"},
			},
			{
				Name:       "events",
				Mode:       WatchMode,
				Namespaces: []string{"default"},
			},
		}

		err := rCfg.Validate()
		Expect(err).NotTo(HaveOccurred())

		consumer := newMockLogConsumer()
		r, err := newReceiver(
			receivertest.NewNopCreateSettings(),
			rCfg,
			consumer,
		)
		Expect(r).NotTo(BeNil())
		Expect(err).NotTo(HaveOccurred())

		ctx := context.Background()
		Expect(r.Start(ctx, componenttest.NewNopHost())).NotTo(HaveOccurred())

		time.Sleep(time.Millisecond * 100)

		pod1 := generatePod("pod1", "default", map[string]interface{}{
			"environment": "production",
		}, "1234")
		pod2 := generatePod("pod2", "default", map[string]interface{}{
			"environment": "test",
		}, "5678")
		pod3 := generatePod("pod3", "default_ignore", map[string]interface{}{
			"environment": "production",
		}, "9012")
		mockClient.createPods(pod1, pod2, pod3)
		time.Sleep(time.Second * 1)

		// The pods mentioned by the events below have all been reported yet.
		// We expect only one log per event.
		mockClient.createEvents(
			generateEvent("event1", "default", map[string]interface{}{
				"environment": "production",
			}, pod1),
			generateEvent("event2", "default", map[string]interface{}{
				"environment": "production",
			}, pod2),
		)

		time.Sleep(time.Second * 1)

		Eventually(func(g Gomega) {
			logRecords := getLogRecords(consumer.Logs())

			g.Expect(logRecords).Should(ContainElement(matchWatchEvent(watch.Added, toObjectReference(pod1))))
			g.Expect(logRecords).Should(ContainElement(matchWatchEvent(watch.Added, toObjectReference(pod2))))
		}, 1*time.Minute, 1*time.Second).Should(Succeed())

		// TODO Improve tests using a deployment and check that all needed resourceVersions of pods and replicasets
		// are fetched
	})
})

func getLogRecords(logs []plog.Logs) []plog.LogRecord {
	logRecords := make([]plog.LogRecord, 0)

	for g := 0; g < len(logs); g++ {
		for i := 0; i < logs[g].ResourceLogs().Len(); i++ {
			resourceLogs := logs[g].ResourceLogs().At(i)
			for j := 0; j < resourceLogs.ScopeLogs().Len(); j++ {
				scopeLogs := resourceLogs.ScopeLogs().At(j)
				for k := 0; k < scopeLogs.LogRecords().Len(); k++ {
					logRecords = append(logRecords, scopeLogs.LogRecords().At(k))
				}
			}
		}
	}

	return logRecords
}

func matchWatchEvent(eventType watch.EventType, objectReference *corev1.ObjectReference) WatchEventMatcher {
	return WatchEventMatcher{
		ExpectedEventType:       eventType,
		ExpectedObjectReference: objectReference,
	}
}

type WatchEventMatcher struct {
	ExpectedEventType       watch.EventType
	ExpectedObjectReference *corev1.ObjectReference
}

func (wem WatchEventMatcher) Match(actual interface{}) (success bool, err error) {
	logRecord := actual.(plog.LogRecord)

	eventRaw := logRecord.Body().AsRaw().(map[string]interface{})
	eventType := eventRaw["type"]

	if eventType != string(wem.ExpectedEventType) {
		return false, fmt.Errorf("mismatching event type: expected '%s'; received '%s", wem.ExpectedEventType, eventType)
	}

	eventObject := unstructured.Unstructured{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(eventRaw["object"].(map[string]interface{}), &eventObject); err != nil {
		return false, err
	}

	actualObjectReference := toObjectReference(&eventObject)
	if reflect.DeepEqual(actualObjectReference, wem.ExpectedObjectReference) {
		return true, nil
	}

	return false, fmt.Errorf("mismatching object reference type: expected '%v'; received '%v", wem.ExpectedObjectReference, actualObjectReference)
}

func (wem WatchEventMatcher) FailureMessage(actual interface{}) (message string) {
	return "Actual does not match expected watch.Event"
}

func (wem WatchEventMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return "Actual matches watch.Event"
}
