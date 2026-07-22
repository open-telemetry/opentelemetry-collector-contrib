// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package procx // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/sumologicextension/internal/procx"

import (
	"github.com/shirou/gopsutil/v4/process"
	"go.uber.org/zap"
)

const (
	ApacheTag        SumoTag = "apache"
	DockerTag        SumoTag = "docker"
	ElasticsearchTag SumoTag = "elasticsearch"
	MysqlTag         SumoTag = "mysql"
	NginxTag         SumoTag = "nginx"
	PostgresTag      SumoTag = "postgres"
	RabbitmqTag      SumoTag = "rabbitmq"
	RedisTag         SumoTag = "redis"
	TomcatTag        SumoTag = "tomcat"
	KafkaTag         SumoTag = "kafka"
	MongoDBTag       SumoTag = "mongodb"
	CassandraTag     SumoTag = "cassandra"
	JmxTag           SumoTag = "jmx"
	ActiveMQTag      SumoTag = "activemq"
	MemcachedTag     SumoTag = "memcached"
	HaproxyTag       SumoTag = "haproxy"
	DockerCETag      SumoTag = "docker-ce"
	MssqlTag         SumoTag = "mssql"
)

type Procx struct {
	logger       *zap.Logger
	getProcesses func() ([]Process, error)
}

func NewProcx(logger *zap.Logger) *Procx {
	return &Procx{
		logger: logger,
		getProcesses: func() ([]Process, error) {
			ps, err := process.Processes()
			ret := make([]Process, len(ps))
			if err != nil {
				return ret, err
			}
			for i := range ps {
				psw := &processWrapper{
					process: ps[i],
				}
				ret[i] = psw
			}
			return ret, err
		},
	}
}
