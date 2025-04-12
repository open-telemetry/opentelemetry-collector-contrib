//go:build windows

package perflib // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/perfcounters/third_party/perflib"

/*
Go bindings for the HKEY_PERFORMANCE_DATA perflib / Performance Counters interface.

# Overview

HKEY_PERFORMANCE_DATA is a low-level alternative to the higher-level PDH library and WMI.
It operates on blocks of counters and only returns raw values without calculating rates
or formatting them, which is exactly what you want for, say, a Prometheus exporter
(not so much for a GUI like Windows Performance Monitor).

Its overhead is much lower than the high-level libraries.

It operates on the same set of perflib providers as PDH and WMI. See this document
for more details on the relationship between the different libraries:
https://msdn.microsoft.com/en-us/library/windows/desktop/aa371643(v=vs.85).aspx

Example C++ source code:
https://msdn.microsoft.com/de-de/library/windows/desktop/aa372138(v=vs.85).aspx

For now, the API is not stable and is probably going to change in future
perflib_exporter releases. If you want to use this library, send the author an email
so we can discuss your requirements and stabilize the API.

# Names

Counter names and help texts are resolved by looking up an index in a name table.
Since Microsoft loves internalization, both names and help texts can be requested
any locally available language.

The library automatically loads the name tables and resolves all identifiers
in English ("Name" and "HelpText" struct members). You can manually resolve
identifiers in a different language by using the NameTable API.

# Performance Counters intro

Windows has a system-wide performance counter mechanism. Most performance counters
are stored as actual counters, not gauges (with some exceptions).
There's additional metadata which defines how the counter should be presented to the user
(for example, as a calculated rate). This library disregards all of the display metadata.

At the top level, there's a number of performance counter objects.
Each object has counter definitions, which contain the metadata for a particular
counter, and either zero or multiple instances. We hide the fact that there are
objects with no instances, and simply return a single null instance.

There's one counter per counter definition and instance (or the object itself, if
there are no instances).

Behind the scenes, every perflib DLL provides one or more objects.
Perflib has a v1 where DLLs are dynamically registered and
unregistered. Some third party applications like VMWare provide their own counters,
but this is, sadly, a rare occurrence.

Different Windows releases have different numbers of counters.

Objects and counters are identified by well-known indices.

Here's an example object with one instance:

	4320 WSMan Quota Statistics [7 counters, 1 instance(s)]
	`-- "WinRMService"
		`-- Total Requests/Second [4322] = 59
		`-- User Quota Violations/Second [4324] = 0
		`-- System Quota Violations/Second [4326] = 0
		`-- Active Shells [4328] = 0
		`-- Active Operations [4330] = 0
		`-- Active Users [4332] = 0
		`-- Process ID [4334] = 928

All "per second" metrics are counters, the rest are gauges.

Another example, with no instance:

	4600 Network QoS Policy [6 counters, 1 instance(s)]
	`-- (default)
		`-- Packets transmitted [4602] = 1744
		`-- Packets transmitted/sec [4604] = 4852
		`-- Bytes transmitted [4606] = 4853
		`-- Bytes transmitted/sec [4608] = 180388626632
		`-- Packets dropped [4610] = 0
		`-- Packets dropped/sec [4612] = 0

You can access the same values using PowerShell's Get-Counter cmdlet
or the Performance Monitor.

	> Get-Counter '\WSMan Quota Statistics(WinRMService)\Process ID'

	Timestamp                 CounterSamples
	---------                 --------------
	1/28/2018 10:18:00 PM     \\DEV\wsman quota statistics(winrmservice)\process id :
							  928

	>  (Get-Counter '\Process(Idle)\% Processor Time').CounterSamples[0] | Format-List *
	[..detailed output...]

Data for some of the objects is also available through WMI:

	> Get-CimInstance Win32_PerfRawData_Counters_WSManQuotaStatistics

	Name                           : WinRMService
	[...]
	ActiveOperations               : 0
	ActiveShells                   : 0
	ActiveUsers                    : 0
	ProcessID                      : 928
	SystemQuotaViolationsPerSecond : 0
	TotalRequestsPerSecond         : 59
	UserQuotaViolationsPerSecond   : 0
*/

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// There's a LittleEndian field in the PERF header - we ought to check it.
//
//nolint:gochecknoglobals
var bo = binary.LittleEndian

// PerfObject Top-level performance object (like "Process").
type PerfObject struct {
	Name string
	// NameIndex Same index you pass to QueryPerformanceData
	NameIndex   uint
	Instances   []*PerfInstance
	CounterDefs []*PerfCounterDef

	Frequency int64

	rawData *perfObjectType
}

// PerfInstance Each object can have multiple instances. For example,
// In case the object has no instances, we return one single PerfInstance with an empty name.
type PerfInstance struct {
	// *not* resolved using a name table
	Name     string
	Counters []*PerfCounter

	rawData         *perfInstanceDefinition
	rawCounterBlock *perfCounterBlock
}

type PerfCounterDef struct {
	Name      string
	NameIndex uint

	// For debugging - subject to removal. CounterType is a perflib
	// implementation detail (see perflib.h) and should not be used outside
	// of this package. We export it so we can show it on /dump.
	CounterType uint32

	// PERF_TYPE_COUNTER (otherwise, it's a gauge)
	IsCounter bool
	// PERF_COUNTER_BASE (base value of a multi-value fraction)
	IsBaseValue bool
	// PERF_TIMER_100NS
	IsNanosecondCounter bool
	HasSecondValue      bool

	rawData *perfCounterDefinition
}

type PerfCounter struct {
	Value       int64
	Def         *PerfCounterDef
	SecondValue int64
}

//nolint:gochecknoglobals
var (
	bufLenGlobal = uint32(400000)
	bufLenCostly = uint32(2000000)
)

// queryRawData Queries the performance counter buffer using RegQueryValueEx, returning raw bytes. See:
// https://msdn.microsoft.com/de-de/library/windows/desktop/aa373219(v=vs.85).aspx
func queryRawData(query string) ([]byte, error) {
	var (
		valType uint32
		buffer  []byte
		bufLen  uint32
	)

	switch query {
	case "Global":
		bufLen = bufLenGlobal
	case "Costly":
		bufLen = bufLenCostly
	default:
		// depends on the number of values requested
		// need make an educated guess
		numCounters := len(strings.Split(query, " "))
		bufLen = uint32(150000 * numCounters)
	}

	buffer = make([]byte, bufLen)

	name, err := windows.UTF16PtrFromString(query)
	if err != nil {
		return nil, fmt.Errorf("failed to encode query string: %w", err)
	}

	for {
		bufLen := uint32(len(buffer))

		err := windows.RegQueryValueEx(
			windows.HKEY_PERFORMANCE_DATA,
			name,
			nil,
			&valType,
			(*byte)(unsafe.Pointer(&buffer[0])),
			&bufLen)

		switch {
		case errors.Is(err, error(windows.ERROR_MORE_DATA)):
			newBuffer := make([]byte, len(buffer)+16384)
			copy(newBuffer, buffer)
			buffer = newBuffer

			continue
		case errors.Is(err, error(windows.ERROR_BUSY)):
			time.Sleep(50 * time.Millisecond)

			continue
		case err != nil:
			var errNo windows.Errno
			if errors.As(err, &errNo) {
				return nil, fmt.Errorf("ReqQueryValueEx failed: %w errno %d", err, uint(errNo))
			}

			return nil, err
		}

		buffer = buffer[:bufLen]

		switch query {
		case "Global":
			if bufLen > bufLenGlobal {
				bufLenGlobal = bufLen
			}
		case "Costly":
			if bufLen > bufLenCostly {
				bufLenCostly = bufLen
			}
		}

		return buffer, nil
	}
}

/*
QueryPerformanceData Query all performance counters that match a given query.

The query can be any of the following:

- "Global" (all performance counters except those Windows marked as costly)

- "Costly" (only the costly ones)

- One or more object indices, separated by spaces ("238 2 5")

Many objects have dependencies - if you query one of them, you often get back
more than you asked for.
*/
func QueryPerformanceData(query string, counterName string) ([]*PerfObject, error) {
	buffer, err := queryRawData(query)
	if err != nil {
		return nil, err
	}

	r := bytes.NewReader(buffer)

	// Read global header

	header := new(perfDataBlock)

	err = header.BinaryReadFrom(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read performance data block for %q with: %w", query, err)
	}

	// Check for "PERF" signature
	if header.Signature != [4]uint16{80, 69, 82, 70} {
		panic("Invalid performance block header")
	}

	// Parse the performance data

	numObjects := int(header.NumObjectTypes)
	numFilteredObjects := 0

	objects := make([]*PerfObject, numObjects)

	objOffset := int64(header.HeaderLength)

	for i := range numObjects {
		_, err := r.Seek(objOffset, io.SeekStart)
		if err != nil {
			return nil, err
		}

		obj := new(perfObjectType)

		err = obj.BinaryReadFrom(r)
		if err != nil {
			return nil, err
		}

		perfCounterName := obj.LookupName()

		if counterName != "" && perfCounterName != counterName {
			objOffset += int64(obj.TotalByteLength)

			continue
		}

		numCounterDefs := int(obj.NumCounters)
		numInstances := int(obj.NumInstances)

		// Perf objects can have no instances. The perflib differentiates
		// between objects with instances and without, but we just create
		// an empty instance in order to simplify the interface.
		if numInstances <= 0 {
			numInstances = 1
		}

		instances := make([]*PerfInstance, numInstances)
		counterDefs := make([]*PerfCounterDef, numCounterDefs)

		objects[i] = &PerfObject{
			Name:        perfCounterName,
			NameIndex:   uint(obj.ObjectNameTitleIndex),
			Instances:   instances,
			CounterDefs: counterDefs,
			Frequency:   obj.PerfFreq,
			rawData:     obj,
		}

		for i := range numCounterDefs {
			def := new(perfCounterDefinition)

			err := def.BinaryReadFrom(r)
			if err != nil {
				return nil, err
			}

			counterDefs[i] = &PerfCounterDef{
				Name:      def.LookupName(),
				NameIndex: uint(def.CounterNameTitleIndex),
				rawData:   def,

				CounterType: def.CounterType,

				IsCounter:           def.CounterType&0x400 == 0x400,
				IsBaseValue:         def.CounterType&0x00030000 == 0x00030000,
				IsNanosecondCounter: def.CounterType&0x00100000 == 0x00100000,
				HasSecondValue:      def.CounterType == PERF_AVERAGE_BULK,
			}
		}

		if obj.NumInstances <= 0 { //nolint:nestif
			blockOffset := objOffset + int64(obj.DefinitionLength)

			if _, err := r.Seek(blockOffset, io.SeekStart); err != nil {
				return nil, err
			}

			_, counters, err := parseCounterBlock(buffer, r, blockOffset, counterDefs)
			if err != nil {
				return nil, err
			}

			instances[0] = &PerfInstance{
				Name:            "",
				Counters:        counters,
				rawData:         nil,
				rawCounterBlock: nil,
			}
		} else {
			instOffset := objOffset + int64(obj.DefinitionLength)

			for i := range numInstances {
				if _, err := r.Seek(instOffset, io.SeekStart); err != nil {
					return nil, err
				}

				inst := new(perfInstanceDefinition)

				if err = inst.BinaryReadFrom(r); err != nil {
					return nil, err
				}

				name, _ := readUTF16StringAtPos(r, instOffset+int64(inst.NameOffset), inst.NameLength)
				pos := instOffset + int64(inst.ByteLength)

				offset, counters, err := parseCounterBlock(buffer, r, pos, counterDefs)
				if err != nil {
					return nil, err
				}

				instances[i] = &PerfInstance{
					Name:     name,
					Counters: counters,
					rawData:  inst,
				}

				instOffset = pos + offset
			}
		}

		if counterName != "" {
			return objects[i : i+1], nil
		}

		// Next perfObjectType
		objOffset += int64(obj.TotalByteLength)
		numFilteredObjects++
	}

	return objects[:numFilteredObjects], nil
}

func parseCounterBlock(b []byte, r io.ReadSeeker, pos int64, defs []*PerfCounterDef) (int64, []*PerfCounter, error) {
	_, err := r.Seek(pos, io.SeekStart)
	if err != nil {
		return 0, nil, err
	}

	block := new(perfCounterBlock)

	err = block.BinaryReadFrom(r)
	if err != nil {
		return 0, nil, err
	}

	counters := make([]*PerfCounter, len(defs))

	for i, def := range defs {
		valueOffset := pos + int64(def.rawData.CounterOffset)
		value := convertCounterValue(def.rawData, b, valueOffset)
		secondValue := int64(0)

		if def.HasSecondValue {
			secondValue = convertCounterValue(def.rawData, b, valueOffset+8)
		}

		counters[i] = &PerfCounter{
			Value:       value,
			Def:         def,
			SecondValue: secondValue,
		}
	}

	return int64(block.ByteLength), counters, nil
}

func convertCounterValue(counterDef *perfCounterDefinition, buffer []byte, valueOffset int64) int64 {
	/*
		We can safely ignore the type since we're not interested in anything except the raw value.
		We also ignore all of the other attributes (timestamp, presentation, multi counter values...)

		See also: winperf.h.

		Here's the most common value for CounterType:

			65536	32bit counter
			65792	64bit counter
			272696320	32bit rate
			272696576	64bit rate

	*/
	switch counterDef.CounterSize {
	case 4:
		return int64(bo.Uint32(buffer[valueOffset:(valueOffset + 4)]))
	case 8:
		return int64(bo.Uint64(buffer[valueOffset:(valueOffset + 8)]))
	default:
		return int64(bo.Uint32(buffer[valueOffset:(valueOffset + 4)]))
	}
}
