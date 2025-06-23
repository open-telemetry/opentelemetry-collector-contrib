// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows && win32service

package supervisor

import (
	"encoding/xml"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	supervisorServiceName = "opampsupervisor"
)

// Test the supervisor as a Windows service.
// The test assumes that the service and respective event source are already created.
//
// To test locally:
// * Build the supervisor and collector binaries. Note you'll need to update the name of the collector binary in the supervisor config file
//   - cd cmd/opampsupervisor; go build
//   - make otelcontribcol
//
// * Install the Windows service
//   - New-Service -Name "opampsupervisor" -StartupType "Manual" -BinaryPathName "${PWD}\cmd\opampsupervisor --config ${PWD}\cmd\opampsupervisor\supervisor\testdata\supervisor_windows_service_test_config.yaml\"
//
// * Create event log source
//   - eventcreate.exe /t information /id 1 /l application /d "Creating event provider for 'opampsupervisor'" /so opampsupervisor
//

// The test also must be executed with administrative privileges.
func TestSupervisorAsService(t *testing.T) {
	scm, err := mgr.Connect()
	require.NoError(t, err)
	defer scm.Disconnect()

	service, err := scm.OpenService(supervisorServiceName)
	require.NoError(t, err)
	defer service.Close()

	// start up supervisor service
	startTime := time.Now()
	err = service.Start()
	require.NoError(t, err)

	defer func() {
		_, err = service.Control(svc.Stop)
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			status, _ := service.Query()
			return status.State == svc.Stopped
		}, 10*time.Second, 500*time.Millisecond)
	}()

	// wait for supervisor service to start
	require.Eventually(t, func() bool {
		status, _ := service.Query()
		return status.State == svc.Running
	}, 10*time.Second, 500*time.Millisecond)

	// verify supervisor service started & healthy
	// Read the events from the opampsupervisor source and check that they were emitted after the service
	// command started. This is a simple validation that the messages are being logged on the
	// Windows event log.
	cmd := exec.Command("wevtutil.exe", "qe", "Application", "/c:1", "/rd:true", "/f:RenderedXml", "/q:*[System[Provider[@Name='opampsupervisor']]]")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err)

	var e Event
	require.NoError(t, xml.Unmarshal([]byte(out), &e))

	eventTime, err := time.Parse("2006-01-02T15:04:05.9999999Z07:00", e.System.TimeCreated.SystemTime)
	require.NoError(t, err)

	require.True(t, eventTime.After(startTime.In(time.UTC)))
}

// Helper types to read the XML events from the event log using wevtutil
type Event struct {
	XMLName xml.Name `xml:"Event"`
	System  System   `xml:"System"`
	Data    string   `xml:"EventData>Data"`
}

type System struct {
	Provider      Provider    `xml:"Provider"`
	EventID       int         `xml:"EventID"`
	Version       int         `xml:"Version"`
	Level         int         `xml:"Level"`
	Task          int         `xml:"Task"`
	Opcode        int         `xml:"Opcode"`
	Keywords      string      `xml:"Keywords"`
	TimeCreated   TimeCreated `xml:"TimeCreated"`
	EventRecordID int         `xml:"EventRecordID"`
	Execution     Execution   `xml:"Execution"`
	Channel       string      `xml:"Channel"`
	Computer      string      `xml:"Computer"`
}

type Provider struct {
	Name string `xml:"Name,attr"`
}

type TimeCreated struct {
	SystemTime string `xml:"SystemTime,attr"`
}

type Execution struct {
	ProcessID string `xml:"ProcessID,attr"`
	ThreadID  string `xml:"ThreadID,attr"`
}
