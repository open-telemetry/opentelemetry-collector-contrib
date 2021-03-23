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

package dotnet

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dotnetdiagnosticsreceiver/network"
)

// RequestWriter submits the initial request to the dotnet diagnostics backend,
// after which it starts sending us metrics at the specified interval.
// ByteBuffer is swappable for testing.
// For docs on the protocol, see
// https://github.com/dotnet/diagnostics/blob/master/documentation/design-docs/ipc-protocol.md
type RequestWriter struct {
	w           io.Writer
	intervalSec int
	// providerNames (aka event sources) indicate which counter groups to get metrics for. e.g. "System.Runtime"
	providerNames []string
}

func NewRequestWriter(w io.Writer, intervalSec int, providerNames ...string) *RequestWriter {
	return &RequestWriter{w: w, intervalSec: intervalSec, providerNames: providerNames}
}

func (w *RequestWriter) SendRequest() error {
	req := w.createRequest()
	_, err := w.w.Write(req)
	return err
}

// The following constants come from:
// https://github.com/dotnet/diagnostics/blob/master/src/Microsoft.Diagnostics.NETCore.Client/DiagnosticsIpc/IpcCommands.cs

// Corresponds to DiagnosticsServerCommandSet.EventPipe
const eventPipeCommand = 2

// Corresponds to EventPipeCommandId.CollectTracing2
const collectTracing2CommandID = 3

func (w *RequestWriter) createRequest() []byte {
	cfgReq := newConfigRequest(w.intervalSec, w.providerNames...)
	payload := cfgReq.serialize()
	hdr := &requestHeader{
		commandSet: eventPipeCommand,
		commandID:  collectTracing2CommandID,
	}
	hdrBytes := hdr.serialize(len(payload))
	return append(hdrBytes, payload...)
}

// requestHeader
type requestHeader struct {
	commandSet byte
	commandID  byte
}

const magic = "DOTNET_IPC_V1"
const magicTerminated = magic + "\000"

const requestHeaderSize = 20

func (h requestHeader) serialize(payloadSize int) []byte {
	buf := &bytes.Buffer{}
	// no need to handle errors because Write always returns a nil error
	_, _ = buf.Write([]byte(magicTerminated))
	totSize := uint16(requestHeaderSize + payloadSize)
	_ = binary.Write(buf, network.ByteOrder, totSize)
	_, _ = buf.Write([]byte{h.commandSet, h.commandID, 0, 0})
	return buf.Bytes()
}

// configRequest
type configRequest struct {
	circularBufferSizeInMB int32
	format                 serializationFormat
	requestRundown         bool
	providers              []provider
}

type serializationFormat uint32

const netTrace = 1

func newConfigRequest(intervalSec int, providerNames ...string) configRequest {
	return configRequest{
		circularBufferSizeInMB: 10,
		format:                 netTrace,
		requestRundown:         false,
		providers:              createProviders(intervalSec, providerNames...),
	}
}

func (c configRequest) serialize() []byte {
	buf := &bytes.Buffer{}
	_ = binary.Write(buf, network.ByteOrder, c.circularBufferSizeInMB)
	_ = binary.Write(buf, network.ByteOrder, c.format)
	_ = binary.Write(buf, network.ByteOrder, c.requestRundown)
	size := int32(len(c.providers))
	_ = binary.Write(buf, network.ByteOrder, size)
	for _, p := range c.providers {
		p.serialize(buf)
	}
	return buf.Bytes()
}

// provider
type provider struct {
	name       string
	eventLevel uint32
	keywords   int64
	args       providerArgs
}

func createProviders(intervalSec int, names ...string) (ps []provider) {
	for _, name := range names {
		ps = append(ps, createProvider(name, intervalSec))
	}
	return
}

// from https://docs.microsoft.com/en-us/dotnet/api/system.diagnostics.tracing.eventlevel
const verboseEventLevel = 5

func createProvider(name string, intervalSec int) provider {
	s := strconv.Itoa(intervalSec)
	return provider{
		name:       name,
		eventLevel: verboseEventLevel,
		keywords:   math.MaxInt64,
		args:       providerArgs{"EventCounterIntervalSec": s},
	}
}

func (p provider) serialize(buf *bytes.Buffer) {
	_ = binary.Write(buf, network.ByteOrder, p.keywords)
	_ = binary.Write(buf, network.ByteOrder, p.eventLevel)
	network.WriteUTF16String(buf, p.name)
	network.WriteUTF16String(buf, p.args.String())
}

// providerArgs
type providerArgs map[string]string

func (a providerArgs) String() string {
	keys := make([]string, 0, len(a))
	for k := range a {
		keys = append(keys, k)
	}
	// sorted for testing
	sort.Strings(keys)
	s := ""
	for _, k := range keys {
		if len(s) > 0 {
			s += ";"
		}
		v := a[k]
		s += fmt.Sprintf("%s=%s", escapeArg(k), escapeArg(v))
	}
	return s
}

func escapeArg(s string) string {
	if strings.ContainsAny(s, ";=") {
		return fmt.Sprintf("%q", s)
	}
	return s
}
