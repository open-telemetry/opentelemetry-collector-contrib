// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows
// +build !windows

package podmanreceiver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/zap"
)

func tmpSock(t *testing.T) (net.Listener, string) {
	f, err := os.CreateTemp(os.TempDir(), "testsock")
	if err != nil {
		t.Fatal(err)
	}
	addr := f.Name()
	os.Remove(addr)

	listener, err := net.Listen("unix", addr)
	if err != nil {
		t.Fatal(err)
	}

	return listener, addr
}

func TestStats(t *testing.T) {
	// stats sample
	statsExample := `{"Error":null,"Stats":[{"AvgCPU":42.04781177856639,"ContainerID":"e6af5805edae6c950003abd5451808b277b67077e400f0a6f69d01af116ef014","Name":"charming_sutherland","PerCPU":null,"CPU":42.04781177856639,"CPUNano":309165846000,"CPUSystemNano":54515674,"SystemNano":1650912926385978706,"MemUsage":27717632,"MemLimit":7942234112,"MemPerc":0.34899036730888044,"NetInput":430,"NetOutput":330,"BlockInput":0,"BlockOutput":0,"PIDs":118,"UpTime":309165846000,"Duration":309165846000}]}`

	listener, addr := tmpSock(t)
	defer listener.Close()
	defer os.Remove(addr)

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/stats") {
			_, err := w.Write([]byte(statsExample))
			assert.NoError(t, err)
		} else {
			_, err := w.Write([]byte{})
			assert.NoError(t, err)
		}
	}))
	srv.Listener = listener
	srv.Start()
	defer srv.Close()

	config := &Config{
		Endpoint: fmt.Sprintf("unix://%s", addr),
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			// default timeout
			Timeout: 5 * time.Second,
		},
	}

	cli, err := newLibpodClient(zap.NewNop(), config)
	assert.NotNil(t, cli)
	assert.Nil(t, err)

	expectedStats := containerStats{
		AvgCPU:        42.04781177856639,
		ContainerID:   "e6af5805edae6c950003abd5451808b277b67077e400f0a6f69d01af116ef014",
		Name:          "charming_sutherland",
		PerCPU:        nil,
		CPU:           42.04781177856639,
		CPUNano:       309165846000,
		CPUSystemNano: 54515674,
		SystemNano:    1650912926385978706,
		MemUsage:      27717632,
		MemLimit:      7942234112,
		MemPerc:       0.34899036730888044,
		NetInput:      430,
		NetOutput:     330,
		BlockInput:    0,
		BlockOutput:   0,
		PIDs:          118,
		UpTime:        309165846000 * time.Nanosecond,
		Duration:      309165846000,
	}

	stats, err := cli.stats(context.Background(), nil)
	assert.NoError(t, err)
	assert.Equal(t, expectedStats, stats[0])
}

func TestStatsError(t *testing.T) {
	// If the stats request fails, the API returns the following Error structure: https://docs.podman.io/en/latest/_static/api.html#operation/ContainersStatsAllLibpod
	// For example, if we query the stats with an invalid container ID, the API returns the following message
	statsError := `{"Error":{},"Stats":null}`

	listener, addr := tmpSock(t)
	defer listener.Close()
	defer os.Remove(addr)

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/stats") {
			_, err := w.Write([]byte(statsError))
			assert.NoError(t, err)
		} else {
			_, err := w.Write([]byte{})
			assert.NoError(t, err)
		}
	}))
	srv.Listener = listener
	srv.Start()
	defer srv.Close()

	config := &Config{
		Endpoint: fmt.Sprintf("unix://%s", addr),
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			// default timeout
			Timeout: 5 * time.Second,
		},
	}

	cli, err := newLibpodClient(zap.NewNop(), config)
	assert.NotNil(t, cli)
	assert.Nil(t, err)

	stats, err := cli.stats(context.Background(), nil)
	assert.Nil(t, stats)
	assert.EqualError(t, err, errNoStatsFound.Error())
}

func TestList(t *testing.T) {
	// list sample
	listExample := `[{"AutoRemove":false,"Command":["nginx","-g","daemon off;"],"Created":"2022-05-28T11:25:35.999277074+02:00","CreatedAt":"","Exited":false,"ExitedAt":-62135596800,"ExitCode":0,"Id":"aa3e2040dee22a369d2c8f0b712a5ff045e8f1ce47f5e943426ce6664e3ef379","Image":"library/nginxy:latest","ImageID":"12766a6745eea133de9fdcd03ff720fa971fdaf21113d4bc72b417c123b15619","IsInfra":false,"Labels":{"maintainer":"someone"},"Mounts":[],"Names":["sharp_curran"],"Namespaces":{},"Networks":[],"Pid":7892,"Pod":"","PodName":"","Ports":null,"Size":null,"StartedAt":1653729936,"State":"running","Status":""}]`

	listener, addr := tmpSock(t)
	defer listener.Close()
	defer os.Remove(addr)

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/containers/json") {
			_, err := w.Write([]byte(listExample))
			assert.NoError(t, err)
		} else {
			_, err := w.Write([]byte{})
			assert.NoError(t, err)
		}
	}))
	srv.Listener = listener
	srv.Start()
	defer srv.Close()

	config := &Config{
		Endpoint: fmt.Sprintf("unix://%s", addr),
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			// default timeout
			Timeout: 5 * time.Second,
		},
	}

	cli, err := newLibpodClient(zap.NewNop(), config)
	assert.NotNil(t, cli)
	assert.Nil(t, err)

	expectedContainer := container{

		AutoRemove: false,
		Command:    []string{"nginx", "-g", "daemon off;"},
		Created:    "2022-05-28T11:25:35.999277074+02:00",
		CreatedAt:  "",
		Exited:     false,
		ExitedAt:   -62135596800,
		ExitCode:   0,
		ID:         "aa3e2040dee22a369d2c8f0b712a5ff045e8f1ce47f5e943426ce6664e3ef379",
		Image:      "library/nginxy:latest",
		ImageID:    "12766a6745eea133de9fdcd03ff720fa971fdaf21113d4bc72b417c123b15619",
		IsInfra:    false,
		Labels:     map[string]string{"maintainer": "someone"},
		Mounts:     []string{},
		Names:      []string{"sharp_curran"},
		Namespaces: map[string]string{},
		Networks:   []string{},
		Pid:        7892,
		Pod:        "",
		PodName:    "",
		Ports:      nil,
		Size:       nil,
		StartedAt:  1653729936,
		State:      "running",
		Status:     "",
	}

	containers, err := cli.list(context.Background(), nil)
	assert.NoError(t, err)
	assert.Equal(t, expectedContainer, containers[0])
}

func TestEvents(t *testing.T) {
	// event samples
	eventsExample := []string{
		`{"status":"start","id":"49a4c52afb06e6b36b2941422a0adf47421dbfbf40503dbe17bd56b4570b6681","from":"docker.io/library/httpd:latest","Type":"container","Action":"start","Actor":{"ID":"49a4c52afb06e6b36b2941422a0adf47421dbfbf40503dbe17bd56b4570b6681","Attributes":{"containerExitCode":"0","image":"docker.io/library/httpd:latest","name":"vigilant_jennings"}},"scope":"local","time":1655230086,"timeNano":1655230086294801585}`,
		`{"status":"died","id":"d5c43c6954e4bfe62170c75f9f18f81da644bd35bfd22dbfafda349192d4940a","from":"docker.io/library/nginx:latest","Type":"container","Action":"died","Actor":{"ID":"d5c43c6954e4bfe62170c75f9f18f81da644bd35bfd22dbfafda349192d4940a","Attributes":{"containerExitCode":"0","image":"docker.io/library/nginx:latest","name":"relaxed_mccarthy"}},"scope":"local","time":1655653026,"timeNano":1655653026340832435}`,
	}

	listener, addr := tmpSock(t)
	defer listener.Close()
	defer os.Remove(addr)

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "events") {
			// write stream events
			for _, event := range eventsExample {
				_, err := w.Write([]byte(event))
				assert.NoError(t, err)
			}
			// empty write to generate io.EOF error
			_, err := w.Write([]byte{})
			assert.NoError(t, err)
		} else {
			_, err := w.Write([]byte{})
			assert.NoError(t, err)
		}
	}))
	srv.Listener = listener
	srv.Start()
	defer srv.Close()

	config := &Config{
		Endpoint: fmt.Sprintf("unix://%s", addr),
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			// default timeout
			Timeout: 5 * time.Second,
		},
	}

	cli, err := newLibpodClient(zap.NewNop(), config)
	assert.NotNil(t, cli)
	assert.Nil(t, err)

	expectedEvents := []event{
		{ID: "49a4c52afb06e6b36b2941422a0adf47421dbfbf40503dbe17bd56b4570b6681", Status: "start"},
		{ID: "d5c43c6954e4bfe62170c75f9f18f81da644bd35bfd22dbfafda349192d4940a", Status: "died"},
	}

	events, errs := cli.events(context.Background(), nil)
	var actualEvents []event

loop:
	for {

		select {
		case err := <-errs:
			if err != nil && !errors.Is(err, io.EOF) {
				t.Fatal(err)
			}
			assert.Equal(t, io.EOF, err)
			break loop
		case e := <-events:
			actualEvents = append(actualEvents, e)
		}
	}

	assert.Equal(t, expectedEvents, actualEvents)

}
