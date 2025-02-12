package huaweicloudaomexporter

import "github.com/huaweicloud/huaweicloud-lts-sdk-go/producer"

type ExtendLog struct {
	producer.Log
	Extends []*producer.LogTag `json:"-"`
}
