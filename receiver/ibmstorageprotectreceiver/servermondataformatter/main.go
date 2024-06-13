package servermondataformatter

import (
	"fmt"
	"path/filepath"
	"strings"
)

var q_db [][]string
var q_dbspace [][]string
var q_devclass [][]string
var q_job [][]string
var q_process [][]string
var q_replication [][]string
var q_session [][]string
var q_stgpool [][]string
var q_stgpooldir [][]string
var q_log [][]string

var locationOfFiles = ""     //Provide path to directory containing unstructed files from Servermon ex - "/Users/shubhampandey/Documents/sp-diagnostic-tool-master_Latest_Naming_Convention/data/src"
var destinationlocation = "" // Provide path to file where you want to store the structured data as result e -  "/Users/shubhampandey/Documents/sp-diagnostic-tool-master_Latest_Naming_Convention/data/dst/dest.json"

func DataTransformerScript() {
	// fmt.Println("************************  Started parsing 10-min-show files  ************************")
	fmt.Println("Src directory : ", locationOfFiles)
	fmt.Println("Dst directory : ", destinationlocation)

	locations := Read10min(locationOfFiles)
	lst := ListQueries()

	for _, path := range locations {
		for _, query := range lst {

			data := ExtractData(query, path)
			headers := GetColumns(query)
			if len(data) == 0 {
				continue
			}
			// Query for replication
			if query == "QUERY REPLICATION * STATUS=RUNNING FORMAT=DETAILED" {

				filename := filepath.Base(path)

				// Remove the .xml extension
				fileNameWithoutExtension := strings.TrimSuffix(filename, ".xml")
				SerializeToJson(query, data, headers, destinationlocation, fileNameWithoutExtension)
			}
			Factory(query, data, headers)

		}

	}
	// fmt.Println("************************  Done with 10min files  ************************")

	ScriptTwentyMin(locationOfFiles, destinationlocation)
}

func Factory(query string, data [][]string, headers []string) {
	switch query {
	case "QUERY REPLICATION * STATUS=RUNNING FORMAT=DETAILED":
		q_replication = append(q_replication, data...)
	case "QUERY PROCESS":
		q_process = append(q_process, data...)
	case "QUERY DB FORMAT=DETAILED":
		q_db = append(q_db, data...)
	case "QUERY DBSPACE FORMAT=DETAILED":
		q_dbspace = append(q_dbspace, data...)
	case "QUERY LOG FORMAT=DETAILED":
		q_log = append(q_log, data...)
	case "QUERY DEVCLASS FORMAT=DETAILED":
		q_devclass = append(q_devclass, data...)
	case "QUERY STGPOOL FORMAT=DETAILED":
		q_stgpool = append(q_stgpool, data...)
	case "QUERY STGPOOLDIRECTORY FORMAT=DETAILED":
		q_stgpooldir = append(q_stgpooldir, data...)
	case "QUERY SESSION FORMAT=DETAILED":
		q_session = append(q_session, data...)
	case "QUERY JOB STATUS=RUNNING":
		q_job = append(q_job, data...)
	default:
		return
	}
}
