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
type RequestWriter struct {
	buf         network.ByteBuffer
	w           io.Writer
	intervalSec int
	names       []string
}

func NewRequestWriter(buf network.ByteBuffer, w io.Writer, intervalSec int, names ...string) *RequestWriter {
	return &RequestWriter{buf: buf, w: w, intervalSec: intervalSec, names: names}
}

func (w *RequestWriter) SendRequest() error {
	req, err := w.createRequest()
	if err != nil {
		return err
	}
	_, err = w.w.Write(req)
	return err
}

const eventPipeCommand = 2
const collectTracing2CommandID = 3

func (w *RequestWriter) createRequest() ([]byte, error) {
	cfgReq := newConfigRequest(w.intervalSec, w.names...)
	payload, payloadSize, err := cfgReq.serialize(w.buf)
	if err != nil {
		return nil, err
	}

	hdr := &requestHeader{
		commandSet: eventPipeCommand,
		commandID:  collectTracing2CommandID,
	}
	hdrBytes, err := hdr.serialize(w.buf, payloadSize)
	if err != nil {
		return nil, err
	}

	return append(hdrBytes, payload...), nil
}

// requestHeader
type requestHeader struct {
	commandSet byte
	commandID  byte
}

const magic = "DOTNET_IPC_V1"
const requestHeaderSize = 20

func (h requestHeader) serialize(buf network.ByteBuffer, payloadSize int) ([]byte, error) {
	buf.Reset()

	_, err := buf.Write([]byte(magic + "\000"))
	if err != nil {
		return nil, err
	}

	totSize := uint16(requestHeaderSize + payloadSize)
	err = binary.Write(buf, network.ByteOrder, totSize)
	if err != nil {
		return nil, err
	}

	_, err = buf.Write([]byte{h.commandSet, h.commandID, 0, 0})
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
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

func newConfigRequest(intervalSec int, names ...string) configRequest {
	return configRequest{
		circularBufferSizeInMB: 10,
		format:                 netTrace,
		requestRundown:         false,
		providers:              createProviders(intervalSec, names...),
	}
}

func (c configRequest) serialize(buf network.ByteBuffer) ([]byte, int, error) {
	buf.Reset()
	err := binary.Write(buf, network.ByteOrder, c.circularBufferSizeInMB)
	if err != nil {
		return nil, 0, err
	}

	err = binary.Write(buf, network.ByteOrder, c.format)
	if err != nil {
		return nil, 0, err
	}

	err = binary.Write(buf, network.ByteOrder, c.requestRundown)
	if err != nil {
		return nil, 0, err
	}

	size := int32(len(c.providers))
	err = binary.Write(buf, network.ByteOrder, size)
	if err != nil {
		return nil, 0, err
	}

	for _, p := range c.providers {
		err = p.serialize(buf)
		if err != nil {
			return nil, 0, err
		}
	}
	return buf.Bytes(), buf.Len(), nil
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

func (p provider) serialize(buf network.ByteBuffer) error {
	err := binary.Write(buf, network.ByteOrder, p.keywords)
	if err != nil {
		return err
	}

	err = binary.Write(buf, network.ByteOrder, p.eventLevel)
	if err != nil {
		return err
	}

	err = network.WriteUTF16String(buf, p.name)
	if err != nil {
		return err
	}

	err = network.WriteUTF16String(buf, p.args.String())
	if err != nil {
		return err
	}

	return nil
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
