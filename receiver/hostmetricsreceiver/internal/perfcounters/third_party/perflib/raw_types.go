//go:build windows

package perflib // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/perfcounters/third_party/perflib"

import (
	"encoding/binary"
	"io"

	"golang.org/x/sys/windows"
)

/*
perfDataBlock
See: https://msdn.microsoft.com/de-de/library/windows/desktop/aa373157(v=vs.85).aspx

	typedef struct _PERF_DATA_BLOCK {
	  WCHAR         Signature[4];
	  DWORD         LittleEndian;
	  DWORD         Version;
	  DWORD         Revision;
	  DWORD         TotalByteLength;
	  DWORD         HeaderLength;
	  DWORD         NumObjectTypes;
	  DWORD         DefaultObject;
	  SYSTEMTIME    SystemTime;
	  LARGE_INTEGER PerfTime;
	  LARGE_INTEGER PerfFreq;
	  LARGE_INTEGER PerfTime100nSec;
	  DWORD         SystemNameLength;
	  DWORD         SystemNameOffset;
	} PERF_DATA_BLOCK;
*/
type perfDataBlock struct {
	Signature        [4]uint16
	LittleEndian     uint32
	Version          uint32
	Revision         uint32
	TotalByteLength  uint32
	HeaderLength     uint32
	NumObjectTypes   uint32
	DefaultObject    int32
	SystemTime       windows.Systemtime
	_                uint32 // unknown field
	PerfTime         int64
	PerfFreq         int64
	PerfTime100nSec  int64
	SystemNameLength uint32
	SystemNameOffset uint32
}

func (p *perfDataBlock) BinaryReadFrom(r io.Reader) error {
	return binary.Read(r, bo, p)
}

/*
perfObjectType
See: https://msdn.microsoft.com/en-us/library/windows/desktop/aa373160(v=vs.85).aspx

	typedef struct _PERF_OBJECT_TYPE {
	  DWORD         TotalByteLength;
	  DWORD         DefinitionLength;
	  DWORD         HeaderLength;
	  DWORD         ObjectNameTitleIndex;
	  LPWSTR        ObjectNameTitle;
	  DWORD         ObjectHelpTitleIndex;
	  LPWSTR        ObjectHelpTitle;
	  DWORD         DetailLevel;
	  DWORD         NumCounters;
	  DWORD         DefaultCounter;
	  DWORD         NumInstances;
	  DWORD         CodePage;
	  LARGE_INTEGER PerfTime;
	  LARGE_INTEGER PerfFreq;
	} PERF_OBJECT_TYPE;
*/
type perfObjectType struct {
	TotalByteLength      uint32
	DefinitionLength     uint32
	HeaderLength         uint32
	ObjectNameTitleIndex uint32
	ObjectNameTitle      uint32
	ObjectHelpTitleIndex uint32
	ObjectHelpTitle      uint32
	DetailLevel          uint32
	NumCounters          uint32
	DefaultCounter       int32
	NumInstances         int32
	CodePage             uint32
	PerfTime             int64
	PerfFreq             int64
}

func (p *perfObjectType) BinaryReadFrom(r io.Reader) error {
	return binary.Read(r, bo, p)
}

/*
perfCounterDefinition
See: https://msdn.microsoft.com/en-us/library/windows/desktop/aa373150(v=vs.85).aspx

	typedef struct _PERF_COUNTER_DEFINITION {
	  DWORD  ByteLength;
	  DWORD  CounterNameTitleIndex;
	  LPWSTR CounterNameTitle;
	  DWORD  CounterHelpTitleIndex;
	  LPWSTR CounterHelpTitle;
	  LONG   DefaultScale;
	  DWORD  DetailLevel;
	  DWORD  CounterType;
	  DWORD  CounterSize;
	  DWORD  CounterOffset;
	} PERF_COUNTER_DEFINITION;
*/
type perfCounterDefinition struct {
	ByteLength            uint32
	CounterNameTitleIndex uint32
	CounterNameTitle      uint32
	CounterHelpTitleIndex uint32
	CounterHelpTitle      uint32
	DefaultScale          int32
	DetailLevel           uint32
	CounterType           uint32
	CounterSize           uint32
	CounterOffset         uint32
}

func (p *perfCounterDefinition) BinaryReadFrom(r io.Reader) error {
	return binary.Read(r, bo, p)
}

func (p *perfCounterDefinition) LookupName() string {
	return CounterNameTable.LookupString(p.CounterNameTitleIndex)
}

/*
perfCounterBlock
See: https://msdn.microsoft.com/en-us/library/windows/desktop/aa373147(v=vs.85).aspx

	typedef struct _PERF_COUNTER_BLOCK {
	  DWORD ByteLength;
	} PERF_COUNTER_BLOCK;
*/
type perfCounterBlock struct {
	ByteLength uint32
}

func (p *perfCounterBlock) BinaryReadFrom(r io.Reader) error {
	return binary.Read(r, bo, p)
}

/*
perfInstanceDefinition
See: https://msdn.microsoft.com/en-us/library/windows/desktop/aa373159(v=vs.85).aspx

	typedef struct _PERF_INSTANCE_DEFINITION {
	  DWORD ByteLength;
	  DWORD ParentObjectTitleIndex;
	  DWORD ParentObjectInstance;
	  DWORD UniqueID;
	  DWORD NameOffset;
	  DWORD NameLength;
	} PERF_INSTANCE_DEFINITION;
*/
type perfInstanceDefinition struct {
	ByteLength             uint32
	ParentObjectTitleIndex uint32
	ParentObjectInstance   uint32
	UniqueID               uint32
	NameOffset             uint32
	NameLength             uint32
}

func (p *perfInstanceDefinition) BinaryReadFrom(r io.Reader) error {
	return binary.Read(r, bo, p)
}
