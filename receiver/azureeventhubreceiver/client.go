// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"
import (
	"context"
	"fmt"

	eventhub "github.com/Azure/azure-event-hubs-go/v3"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
)

type client struct {
	logger   *zap.Logger
	consumer consumer.Logs
	config   *Config
	obsrecv  *obsreport.Receiver
	hub      *eventhub.Hub
}

func (c *client) Start(ctx context.Context, host component.Host) error {
	storageClient, err := adapter.GetStorageClient(ctx, c.config.ID(), component.KindReceiver, host)
	if err != nil {
		return err
	}
	hub, err := eventhub.NewHubFromConnectionString(c.config.Connection, eventhub.HubWithOffsetPersistence(&storageCheckpointPersister{storageClient: storageClient}))
	c.hub = hub

	if err != nil {
		return err
	}

	if c.config.Partition == "" {
		// listen to each partition of the Event Hub
		var runtimeInfo *eventhub.HubRuntimeInformation
		runtimeInfo, err = c.hub.GetRuntimeInformation(ctx)
		if err != nil {
			return err
		}

		for _, partitionID := range runtimeInfo.PartitionIDs {
			err = c.setUpOnePartition(ctx, partitionID, false)
			if err != nil {
				return err
			}
		}
	} else {
		err = c.setUpOnePartition(ctx, c.config.Partition, true)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *client) setUpOnePartition(ctx context.Context, partitionID string, applyOffset bool) error {
	offsetOption := eventhub.ReceiveWithLatestOffset()
	if applyOffset && c.config.Offset != "" {
		offsetOption = eventhub.ReceiveWithStartingOffset(c.config.Offset)
	}

	handle, err := c.hub.Receive(ctx, partitionID, c.handle, offsetOption)
	if err != nil {
		return err
	}
	go func() {
		<-handle.Done()
		err := handle.Err()
		if err != nil {
			c.logger.Error("Error reported by event hub", zap.Error(err))
		}
	}()

	return nil
}

func (c *client) handle(ctx context.Context, event *eventhub.Event) error {
	c.obsrecv.StartLogsOp(ctx)
	l := plog.NewLogs()
	lr := l.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	lr.Body().SetBytesVal(pcommon.NewImmutableByteSlice(event.Data))
	for k, v := range event.Properties {
		var newValue pcommon.Value
		newValue, err := newValueFromRaw(v)
		if err != nil {
			c.logger.Warn("unsupported property", zap.Error(err))
		} else {
			lr.Attributes().Insert(k, newValue)
		}
	}
	if event.SystemProperties.EnqueuedTime != nil {
		lr.SetTimestamp(pcommon.NewTimestampFromTime(*event.SystemProperties.EnqueuedTime))
	}
	consumerErr := c.consumer.ConsumeLogs(ctx, l)
	c.obsrecv.EndLogsOp(ctx, "azureeventhub", 1, consumerErr)
	return consumerErr
}

func (c *client) Shutdown(ctx context.Context) error {
	if c.hub == nil {
		return nil
	}
	return c.hub.Close(ctx)
}

// copied from pcommon code.
func newValueFromRaw(iv interface{}) (pcommon.Value, error) {
	switch tv := iv.(type) {
	case nil:
		return pcommon.NewValueEmpty(), nil
	case string:
		return pcommon.NewValueString(tv), nil
	case int:
		return pcommon.NewValueInt(int64(tv)), nil
	case int8:
		return pcommon.NewValueInt(int64(tv)), nil
	case int16:
		return pcommon.NewValueInt(int64(tv)), nil
	case int32:
		return pcommon.NewValueInt(int64(tv)), nil
	case int64:
		return pcommon.NewValueInt(tv), nil
	case uint:
		return pcommon.NewValueInt(int64(tv)), nil
	case uint8:
		return pcommon.NewValueInt(int64(tv)), nil
	case uint16:
		return pcommon.NewValueInt(int64(tv)), nil
	case uint32:
		return pcommon.NewValueInt(int64(tv)), nil
	case uint64:
		return pcommon.NewValueInt(int64(tv)), nil
	case float32:
		return pcommon.NewValueDouble(float64(tv)), nil
	case float64:
		return pcommon.NewValueDouble(tv), nil
	case bool:
		return pcommon.NewValueBool(tv), nil
	case []byte:
		return pcommon.NewValueBytes(pcommon.NewImmutableByteSlice(tv)), nil
	case map[string]interface{}:
		mv := pcommon.NewValueMap()
		pcommon.NewMapFromRaw(tv).CopyTo(mv.MapVal())
		return mv, nil
	case []interface{}:
		av := pcommon.NewValueSlice()
		pcommon.NewSliceFromRaw(tv).CopyTo(av.SliceVal())
		return av, nil
	default:
		return pcommon.NewValueEmpty(), fmt.Errorf("unsupported value type %T>", tv)
	}
}
