package timeutils

// #define _XOPEN_SOURCE
// #define _DEFAULT_SOURCE
// #include <stdlib.h>
// #include <time.h>
import (
	"C"
)
import (
	"errors"
	"fmt"
	"time"
	"unsafe"
)

func tm2Time(tm C.struct_tm) time.Time {
	return time.Date(
		int(tm.tm_year+1900),
		time.Month(tm.tm_mon+1),
		int(tm.tm_mday),
		int(tm.tm_hour),
		int(tm.tm_min),
		int(tm.tm_sec),
		0,
		time.FixedZone((time.Duration(tm.tm_gmtoff)*time.Second).String(), int(tm.tm_gmtoff)),
	)
}

// Wrap libc's strptime for use in TestTimeParserStrptimeCgo
func C_strptime(s, format string) (time.Time, error) {
	cformat := C.CString(format)
	defer C.free(unsafe.Pointer(cformat))

	cs := C.CString(s)
	defer C.free(unsafe.Pointer(cs))

	var tm C.struct_tm
	out := C.strptime(cs, cformat, &tm)

	t := tm2Time(tm)
	// ts := int64(C.mktime(&tm))

	if out == nil {
		return time.Time{}, errors.New("strptime failed to parse the whole string")
	} else if unsafe.Pointer(out) != unsafe.Add(unsafe.Pointer(cs), len(s)) {
		return t, fmt.Errorf("strptime failed to parse: remainder %q", C.GoString(out))
	}
	return t, nil
}
