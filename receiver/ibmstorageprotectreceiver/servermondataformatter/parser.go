package servermondataformatter

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func extractAllBodyTags(dataFile string) []string {
	patternToExtractBodyTags := regexp.MustCompile(`(?s)<BODY>(.*?)</BODY>`)
	matches := patternToExtractBodyTags.FindAllString(dataFile, -1)
	return matches
}

func extractAllUnstructuredTables(data string) []string {
	patternToExtractTables := regexp.MustCompile(`(?s)Thread \d+ .*?Total\s+[\d.]+`)
	tables := patternToExtractTables.FindAllString(data, -1)
	return tables
}

func fetchRowHeadingAsList(table string) []string {
	var rowHeaderIdx int
	rowHeaderIdx = 0
	lines := strings.Split(strings.TrimSpace(table), "\n")
	for rowHeaderIdx < len(lines) && !strings.HasPrefix(lines[rowHeaderIdx], "Operation") {
		rowHeaderIdx++
	}
	if rowHeaderIdx >= len(lines) {
		return nil
	}
	header := strings.Fields(lines[rowHeaderIdx])
	header[len(header)-2] += " " + header[len(header)-1]
	header = header[:len(header)-1]
	return header
}

func fetchHeaderIndexes(header []string, lines []string) []int {
	var rowHeaderIdx int
	rowHeaderIdx = 0
	for rowHeaderIdx < len(lines) && !strings.HasPrefix(lines[rowHeaderIdx], "Operation") {
		rowHeaderIdx++
	}
	if rowHeaderIdx >= len(lines) {
		return nil
	}
	var indexes []int
	indexes = make([]int, 0)
	for _, item := range header {
		indexes = append(indexes, strings.Index(lines[rowHeaderIdx], item))
	}
	indexes = append(indexes, len(lines[rowHeaderIdx]))
	for i := 1; i < len(indexes)-1; i++ {
		indexes[i] -= 2
	}
	indexes[6] += 1
	return indexes
}

func extractRows(table string, columnIndexes []int) [][]interface{} {
	lines := strings.Split(strings.TrimSpace(table), "\n")
	stIdx := 1
	for stIdx < len(lines) && !strings.HasPrefix(lines[stIdx], "-") {
		stIdx++
	}
	stIdx++
	var result [][]interface{}
	for _, line := range lines[stIdx:] {
		if strings.HasPrefix(line, "-") {
			continue
		}
		row := make([]interface{}, 0)
		columnOne := strings.TrimSpace(line[columnIndexes[0]:columnIndexes[1]])
		row = append(row, columnOne)
		valuesFrom2ndColumn := strings.Fields(line[len(columnOne):])
		for _, value := range valuesFrom2ndColumn {
			floatValue := 0.0
			fmt.Sscanf(value, "%f", &floatValue)
			row = append(row, floatValue)
		}
		left := 7 - len(valuesFrom2ndColumn)
		for i := 0; i < left; i++ {
			row = append(row, nil)
		}
		result = append(result, row)
		if strings.HasPrefix(line, "Total") {
			break
		}
	}
	// for _, row := range result {
	//  fmt.Println(row)
	// }
	return result
}

func getStructuredObjectTable(table string, filename string) []map[string]interface{} {
	threadPattern := regexp.MustCompile(`Thread \d+ .*?\s+parent=\d+\s+\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}.\d{3}-->\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}.\d{3}`)
	threadStr := threadPattern.FindString(table)
	threadNo := 0
	fmt.Sscanf(strings.Split(threadStr, " ")[1], "%d", &threadNo)
	threadName := strings.Split(threadStr, " ")[2]
	threadParent := 0
	fmt.Sscanf(strings.Split(strings.Split(threadStr, " ")[3], "=")[1], "%d", &threadParent)
	// fields := strings.Fields(threadStr)
	// timeStr := fields[len(fields)-1]

	// threadStrDate := timeStr[:10]
	// threadEndDate := timeStr[26:36]

	// threadStrTime := timeStr[11:23]
	// threadEndTime := timeStr[37:]

	// threadStart := threadStrDate + " " + threadStrTime
	// threadEnd := threadEndDate + " " + threadEndTime

	rowHeader := fetchRowHeadingAsList(table)
	lines := strings.Split(strings.TrimSpace(table), "\n")
	headerIndexes := fetchHeaderIndexes(rowHeader, lines)

	rows := extractRows(table, headerIndexes)
	// fmt.Println(rows[0])
	// fmt.Println(rows[0][3])
	// os.Exit(0)

	res := make([]map[string]interface{}, 0)

	for _, row := range rows {
		if row[6] == nil {
			row[6] = "NaN"
		}
		if row[7] == nil {
			row[7] = "NaN"
		}
		operation := row[0].(string)
		// count := row[1]
		avgTime := row[3]
		minTime := row[4]
		maxTime := row[5]
		entry := map[string]interface{}{
			"resourceMetrics": []map[string]interface{}{
				{
					"scopeMetrics": []map[string]interface{}{
						{
							"scope": map[string]interface{}{
								"name": "SP-Srvrmon",
							},
							"metrics": []map[string]interface{}{
								{
									"name": "SP-Svrmon", //filename[:len(filename)-4],
									"histogram": map[string]interface{}{
										"dataPoints": []map[string]interface{}{
											{
												"attributes": []map[string]interface{}{
													{"key": fmt.Sprintf("%s.%s.AvgTime", threadName, operation), "value": map[string]interface{}{"doubleValue": avgTime}},
													{"key": fmt.Sprintf("%s.%s.MinTime", threadName, operation), "value": map[string]interface{}{"doubleValue": minTime}},
													{"key": fmt.Sprintf("%s.%s.MaxTime", threadName, operation), "value": map[string]interface{}{"doubleValue": maxTime}},
													{"key": "fileName", "value": map[string]interface{}{"stringValue": filename}},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}
		// fmt.Println(entry)
		// os.Exit(0)
		res = append(res, entry)
	}

	return res
}

func extractAllStructuredTablesInOneGo(dataFile string, filename string) []map[string]interface{} {
	bodyTagsData := extractAllBodyTags(dataFile)
	var tableList []string
	for _, tag := range bodyTagsData {
		tables := extractAllUnstructuredTables(tag)
		if len(tables) == 0 {
			continue
		}
		tableList = append(tableList, tables...)
	}
	var allStructuredTables []map[string]interface{}
	for _, table := range tableList {
		structuredTable := getStructuredObjectTable(table, filename)
		allStructuredTables = append(allStructuredTables, structuredTable...)
	}
	return allStructuredTables
}

func ScriptTwentyMin(directory string, destinationLocation string) {
	// fmt.Println("************************  Started parsing 20-min-show files  ************************")
	fmt.Println("Src directory : ", directory)
	fmt.Println("dst directory : ", destinationLocation)

	files, err := ioutil.ReadDir(directory)
	if err != nil {
		fmt.Println("Error reading directory:", err)
		return
	}

	for _, file := range files {
		if strings.HasSuffix(file.Name(), "-20min-show.xml") {
			filepath1 := filepath.Join(directory, file.Name())
			content, err := ioutil.ReadFile(filepath1)
			if err != nil {
				fmt.Println("Error reading file:", err)
				continue
			}

			allStructuredTables := extractAllStructuredTablesInOneGo(string(content), file.Name())
			if allStructuredTables == nil {
				continue
			}
			destinationFile, err := os.OpenFile(destinationLocation, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
			if err != nil {
				fmt.Println("Error opening file:", err)
				continue
			}

			defer destinationFile.Close()

			jsonData, err := json.Marshal(allStructuredTables)
			if err != nil {
				fmt.Println("Error marshalling JSON:", err)
				continue
			}

			jsonString := string(jsonData)
			// jsonString = strings.ReplaceAll(jsonString, "}}]}}]},", "}}]}}]},\n")
			jsonString = strings.ReplaceAll(jsonString, "SP-Srvrmon\"}}]}]},", "SP-Srvrmon\"}}]}]},\n")
			jsonData = []byte(jsonString)

			_, err = destinationFile.Write(jsonData)
			if err != nil {
				fmt.Println("Error writing JSON to file:", err)
				continue
			}

		}
	}
	// fmt.Println("************************  Done with 20-min-show files  ************************")
}
