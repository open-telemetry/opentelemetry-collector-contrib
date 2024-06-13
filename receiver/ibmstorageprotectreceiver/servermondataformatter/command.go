package servermondataformatter

import (
	"encoding/xml"
	"log"
	"os"
	"regexp"
	"strings"
)

// ColumnSTR, columnDate, columnINT, columnCONST, headers, Factory, and rowObj are not implemented in this conversion.

type ADMINCALL struct {
	COMMAND string `xml:"COMMAND"`
	BODY    string `xml:"BODY"`
}

type Root struct {
	XMLName    xml.Name    `xml:"DATASET"`
	ADMINCALLs []ADMINCALL `xml:"ADMINCALL"`
}

func ExtractData(query, xmlFilePath string) [][]string {
	// Read the XML data from the file
	xmlData, err := os.ReadFile(xmlFilePath)
	if err != nil {
		log.Fatalf("Error: File not found at %s\n", xmlFilePath)
	}

	// Replace "<none>" with "none" in XML data
	xmlDataStr := strings.ReplaceAll(string(xmlData), "<none>", "none")

	// Unmarshal modified XML data into Root struct
	var root Root
	if err := xml.Unmarshal([]byte(xmlDataStr), &root); err != nil {
		log.Fatalf("Error unmarshalling modified XML: %v\n", err)
	}

	// Extract data based on query
	var data []string
	for _, admincall := range root.ADMINCALLs {
		if strings.Contains(admincall.COMMAND, query) {
			lines := strings.Split(admincall.BODY, "\n")
			for _, line := range lines {
				if strings.TrimSpace(line) != "" {
					data = append(data, line)
				}
			}
			break
		}
	}
	// return data

	var listOfIndexes [][]int
	var first = true
	var tmp, val int
	Hyph_Index := HyphIndex(data)
	if Hyph_Index == -1 {
		// No data found
		var ans [][]string
		return ans
	}
	hypenatedRow := data[HyphIndex(data)]
	for i, char := range hypenatedRow {
		if char == '-' {
			if first {
				val = i
				tmp = val
				first = false
			}
		} else {
			if !first {
				listOfIndexes = append(listOfIndexes, []int{val, i - 1})
				first = true
			}
		}
	}
	listOfIndexes = append(listOfIndexes, []int{tmp, len(hypenatedRow) - 1})

	// Row calculation
	data = data[HyphIndex(data)+1:]
	return getFormatedList(query, data, listOfIndexes)
}

func getFormatedList(query string, data []string, listOfIndexes [][]int) [][]string {
	start := 0
	s_f := false
	end := 0
	var arr_ver [][]int

	for i := 0; i < len(data); i++ {
		val := data[i]
		char := string(val[RowStartIndex(query)])

		if char >= "0" && char <= "9" {
			if !s_f {
				start = i
				s_f = true
			} else {
				end = i - 1
				arr_ver = append(arr_ver, []int{start, end})
				start = i
			}
		}
	}

	// Include last pair
	arr_ver = append(arr_ver, []int{start, len(data) - 1})
	result := make([][]string, len(arr_ver))
	ver := 0
	for ver != len(arr_ver) {
		row := make([]string, len(listOfIndexes))
		// Iterate horizontally
		for hor := range listOfIndexes {
			s1 := arr_ver[ver][0]
			e1 := arr_ver[ver][1]
			s2 := listOfIndexes[hor][0]
			e2 := listOfIndexes[hor][1]
			ans := ""

			for s1 <= e1 {
				var strr string
				for s2 <= e2 {
					// Convert byte to string before concatenating
					strr += string(data[s1][s2])
					s2++
				}

				if len(strr) == 0 {
					break
				}

				s2 = listOfIndexes[hor][0]
				ans += strr
				s1++
			}

			ans = regexp.MustCompile(`\s+`).ReplaceAllString(ans, " ")
			row[hor] = ans
		}
		result[ver] = row
		ver++
	}

	return result
}

func HyphIndex(data []string) int {
	for i, row := range data {
		if strings.HasPrefix(row, "-") {
			return i
		}
	}
	return -1
}

func ListQueries() []string {
	listofcolumns := []string{
		"QUERY PROCESS",
		"SHOW LOCKS ONLYWAITERS=YES",
		"QUERY DB FORMAT=DETAILED",
		"QUERY DBSPACE FORMAT=DETAILED",
		"QUERY LOG FORMAT=DETAILED",
		"QUERY DEVCLASS FORMAT=DETAILED",
		"QUERY STGPOOL FORMAT=DETAILED",
		"QUERY STGPOOLDIRECTORY FORMAT=DETAILED",
		"QUERY REPLICATION * STATUS=RUNNING FORMAT=DETAILED",
		"SHOW DEDUPTHREAD",
		"SHOW BANNER",
		"SHOW RESQUEUE",
		"SHOW TXNTABLE LOCKDETAIL=NO",
		"QUERY MOUNT",
		"QUERY SESSION FORMAT=DETAILED",
		"SHOW SESSION FORMAT=DETAILED",
		"SHOW THREADS",
		"SHOW JVM",
		"SHOW PRODCONS",
		"QUERY CLOUDREADCACHE",
		"QUERY JOB STATUS=RUNNING",
	}
	return listofcolumns
}

func RowStartIndex(query string) int {
	switch query {
	case "QUERY REPLICATION * STATUS=RUNNING FORMAT=DETAILED":
		return 64
	case "QUERY PROCESS":
		return 7
	case "QUERY DB FORMAT=DETAILED":
		return 13
	case "QUERY DBSPACE FORMAT=DETAILED":
		return 64
	case "QUERY LOG FORMAT=DETAILED":
		return 59
	case "QUERY DEVCLASS FORMAT=DETAILED":
		return 35
	case "QUERY STGPOOL FORMAT=DETAILED":
		return 551
	case "QUERY STGPOOLDIRECTORY FORMAT=DETAILED":
		return 107
	case "QUERY SESSION FORMAT=DETAILED":
		return 5
	case "QUERY JOB STATUS=RUNNING":
		return 9
	default:
		return 0
	}
}
