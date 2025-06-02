package opensearchexporter

import (
	"regexp"
	"strings"
	"time"
)

func FormatIndexName(indexPattern string, t time.Time) string {
	index_format := strings.NewReplacer(
		"%{yyyy}", t.Format("2006"),
		"%{yy}", t.Format("06"),
		"%{mm}", t.Format("01"),
		"%{dd}", t.Format("02"),
		"%{yyyy.mm.dd}", t.Format("2006.01.02"),
		"%{yy.mm.dd}", t.Format("06.01.02"),
		"%{y}", t.Format("06"),
		"%{m}", t.Format("01"),
		"%{d}", t.Format("02"),
	)
	return index_format.Replace(indexPattern)
}

func ContainsIndexPattern(s string) bool {
	re := regexp.MustCompile("%\\{[a-zA-Z0-9.]+\\}")
	return re.MatchString(s)
}
