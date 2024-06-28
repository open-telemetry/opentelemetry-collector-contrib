// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"go.opentelemetry.io/collector/config/configopaque"
)

func TestStreamLoadUrl(t *testing.T) {
	url := streamLoadUrl("http://doris:8030", "otel", "otel_logs")
	fmt.Println(url)
}

func TestCreateMySQLClient(t *testing.T) {
	config := createDefaultConfig().(*Config)
	config.MySQLEndpoint = "127.0.0.1:9030"
	config.Username = "root"
	config.Password = ""

	conn, err := createMySQLClient(config)
	if err != nil {
		t.Error(err)
		return
	}
	defer conn.Close()
}

func TestCreateAndUseDatabase(t *testing.T) {
	config := createDefaultConfig().(*Config)
	config.MySQLEndpoint = "127.0.0.1:9030"
	config.Username = "root"
	config.Password = ""
	config.Database = "otel3"

	conn, err := createMySQLClient(config)
	if err != nil {
		t.Error(err)
		return
	}
	defer conn.Close()

	ctx := context.Background()

	err = createAndUseDatabase(ctx, conn, config)
	if err != nil {
		t.Error(err)
		return
	}

	rows, err := conn.QueryContext(ctx, "SHOW TABLES;")
	if err != nil {
		t.Error(err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var table string
		err = rows.Scan(&table)
		if err != nil {
			t.Error(err)
			return
		}
		t.Error(err)
	}
}

func TestTraceJSON(t *testing.T) {
	trace := &dTrace{}
	marshal, err := json.Marshal(trace)
	if err != nil {
		t.Error(err)
		return
	}

	fmt.Println(string(marshal))
}

func TestTimeFormat(t *testing.T) {
	now := time.Now()
	fmt.Printf("now.Format(TimeFormat): %v\n", now.Format(timeFormat))
}

func TestTimeZone(t *testing.T) {
	timeZone, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Error(err)
		return
	}
	fmt.Println(timeZone)
	fmt.Println(time.Now().In(timeZone).Format(timeFormat))
}

func TestString(t *testing.T) {
	s := configopaque.String("helloworld")
	fmt.Println(string(s))
}
