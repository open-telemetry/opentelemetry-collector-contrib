package perflib

import (
	"strconv"
)

func MapCounterToIndex(name string) string {
	return strconv.Itoa(int(CounterNameTable.LookupIndex(name)))
}
