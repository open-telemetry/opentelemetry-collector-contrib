package main

import (
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/huaweicloudaomexporter"
)

func main() {
	factory := huaweicloudaomexporter.NewFactory()
	fmt.Println(factory)
}

//github.com/open-telemetry/opentelemetry-collector-contrib/exporter/huaweicloudaomexporter v0.109.0
//github.com/open-telemetry/opentelemetry-collector-contrib/exporter/huaweicloudaomexporter v0.109.0
