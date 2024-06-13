package servermondataformatter

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func FindEpochtime(originalTimeStr string) int64 {

	// Remove extra white spaces
	trimmedTimeStr := strings.ReplaceAll(originalTimeStr, " ", "")

	// Defining an example layout for parsing the time
	layout := "01/02/200615:04:05PM"

	parsedTime, err := time.Parse(layout, trimmedTimeStr)
	if err != nil {
		fmt.Println("Error:", err)
		return -1
	}

	// Calculate epoch time in seconds
	epochTime := parsedTime.Unix()

	return epochTime
}

func checkTimeData(input string) bool {
	pattern := `\s(?:AM|PM)`

	// Compile the regex pattern
	regex := regexp.MustCompile(pattern)

	// Search for the pattern in the input string
	if regex.MatchString(input) {
		return true
	}
	return false
}

func DataType(str string) string {

	if checkTimeData(str) {
		return "dateValue"
	}

	comma := 0
	character := 0
	dot := 0
	for _, char := range str {
		if char == ' ' {
			continue
		}
		if char >= '0' && char <= '9' {
			continue
		} else {
			if char == ',' {
				comma++
				break
			} else if char == '.' {
				dot++
			} else {
				character++
			}
		}
	}
	if comma > 0 && character == 0 {
		return "intValue"
	}
	if dot > 0 && character == 0 {
		return "doubleValue"
	}
	if character == 0 && comma == 0 && dot == 0 {
		return "intValue"
	}
	return "stringValue"
}
func SerializeToJson(query string, data [][]string, headers []string, destinationFilePath string, path string) {
	var jsonData []string // Changed the type to string
	for i := 0; i < len(data); i++ {
		// row := make(map[string]interface{})
		attributes := []map[string]interface{}{}
		for j := 0; j < len(headers); j++ {
			if j < len(data[i]) {
				Dtype := DataType(data[i][j])
				if Dtype == "dateValue" {
					value := FindEpochtime(data[i][j])
					attribute := map[string]interface{}{
						"key": headers[j],
						"value": map[string]interface{}{
							"intValue": value,
						},
					}
					attributes = append(attributes, attribute)
				} else if Dtype == "intValue" {
					cleanedValue := strings.ReplaceAll(strings.TrimSpace(data[i][j]), ",", "")
					// Parse the cleaned value into an integer
					value, _ := strconv.Atoi(cleanedValue)
					attribute := map[string]interface{}{
						"key": headers[j],
						"value": map[string]interface{}{
							Dtype: value,
						},
					}
					attributes = append(attributes, attribute)
				} else if Dtype == "doubleValue" {
					cleanedValue := strings.TrimSpace(data[i][j])
					// Parse the cleaned value into an double
					value, _ := strconv.ParseFloat(cleanedValue, 64)
					attribute := map[string]interface{}{
						"key": headers[j],
						"value": map[string]interface{}{
							Dtype: value,
						},
					}
					attributes = append(attributes, attribute)
				} else {
					value := strings.TrimSpace(data[i][j])
					attribute := map[string]interface{}{
						"key": headers[j],
						"value": map[string]interface{}{
							Dtype: value,
						},
					}
					attributes = append(attributes, attribute)
				}
			}
		}

		entry := map[string]interface{}{
			"resourceMetrics": []map[string]interface{}{
				{
					"scopeMetrics": []map[string]interface{}{
						{
							"scope": map[string]interface{}{
								"name": "sample",
							},
							"metrics": []map[string]interface{}{
								{
									"name": path,
									"histogram": map[string]interface{}{
										"dataPoints": []map[string]interface{}{
											{
												"attributes": attributes,
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

		// resource := map[string]interface{}{
		// 	"resource": map[string]interface{}{
		// 		"attributes": attributes,
		// 	},
		// }
		// row["resourceMetrics"] = []map[string]interface{}{resource}
		rowBytes, err := json.Marshal(entry)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		jsonData = append(jsonData, string(rowBytes))
	}
	jsonString := strings.Join(jsonData, "\n")

	// Open the file in append mode, or create it if it doesn't exist
	file, err := os.OpenFile(destinationFilePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		fmt.Println("Enable to open file")
	}
	defer file.Close()
	// Append the JSON data to the file
	_, err = file.WriteString(jsonString + "\n")
	if err != nil {
		fmt.Println("Enable to append data to file")
	}
}
