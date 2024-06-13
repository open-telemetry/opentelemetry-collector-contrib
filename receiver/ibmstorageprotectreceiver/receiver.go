// *******************************************************************************
//  * IBM Confidential
//  * OCO Source Materials
//  * IBM Storage Protect
//  * (C) Copyright IBM Corp. 2024 All Rights Reserved.
//  * The source code for this program is not  published or otherwise divested of
//  * its trade secrets, irrespective of what has been deposited with
//  * the U.S. Copyright Office.
//  ******************************************************************************

package ibmstorageprotectreceiver

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ibmstorageprotectreceiver/servermondataformatter"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/otel/metric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"
)

type ibmstorageprotectreceiver struct {
	input     *fileconsumer.Manager
	id        component.ID
	storageID *component.ID
}

// MeterR and MeterE is used by processors
func MeterR(settings component.TelemetrySettings) metric.Meter {
	return settings.MeterProvider.Meter("otelcol/ibmstorageprotectreceiver")
}

// Starting the receiver component which is done by otel automatically
// refer otelcol-dev/components.go
func (f *ibmstorageprotectreceiver) Start(ctx context.Context, host component.Host) error {
	// fmt.Println("************************  STARTED otlpjsonfilereceiver  ************************")
	storageClient, err := adapter.GetStorageClient(ctx, host, f.storageID, f.id)
	if err != nil {
		return err
	}
	// Calling Data transformer script before starting the receiver
	// Because script contains import statements which caanot be added inside receiver's source code
	// fmt.Println("************************  Started running scripts  ************************")
	servermondataformatter.DataTransformerScript()
	// fmt.Println("************************  Finished running scripts  ************************")

	// From below Start function go to file.go (Ctrl + mouse-click) and inside the consume function modify the
	// source and destination file paths accordingly

	// Since receiver works when there is a change in the file which receiver is watching
	// We are running the datatransfomer before starting the receiver hence, till receiver gets started by the
	// time the file gets static.
	// That's why moving file content from one file to another to make it dynamic

	return f.input.Start(storageClient)
}

func (f *ibmstorageprotectreceiver) Shutdown(_ context.Context) error {
	// fmt.Println("************************  SHUTDOWN otlpjsonfilereceiver  ************************")
	return f.input.Stop()
}
