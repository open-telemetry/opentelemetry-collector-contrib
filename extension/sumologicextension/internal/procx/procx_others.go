// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package procx // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/sumologicextension/internal/procx"

import (
	"fmt"
	"strings"

	"go.uber.org/zap"
)

const (
	ApacheProcessIdentifier             ProcessIdentifier = "apache"
	Apache2ProcessIdentifier            ProcessIdentifier = "apache2"
	HttpdProcessIdentifier              ProcessIdentifier = "httpd"
	DockerProcessIdentifier             ProcessIdentifier = "docker"
	ElasticsearchProcessIdentifier      ProcessIdentifier = "elasticsearch"
	MysqlServerProcessIdentifier        ProcessIdentifier = "mysql-server"
	MysqldProcessIdentifier             ProcessIdentifier = "mysqld"
	NginxProcessIdentifier              ProcessIdentifier = "nginx"
	PostgresProcessIdentifier           ProcessIdentifier = "postgresql"
	Postgres95ProcessIdentifier         ProcessIdentifier = "postgresql-9.5"
	RabbitmqServerProcessIdentifier     ProcessIdentifier = "rabbitmq-server"
	RedisProcessIdentifier              ProcessIdentifier = "redis"
	TomcatProcessIdentifier             ProcessIdentifier = "tomcat"
	KafkaServerStartShProcessIdentifier ProcessIdentifier = "kafka-server-start.sh"
	RedisServerProcessIdentifier        ProcessIdentifier = "redis-server"
	MongodProcessIdentifier             ProcessIdentifier = "mongod"
	CassandraProcessIdentifier          ProcessIdentifier = "cassandra"
	JmxProcessIdentifier                ProcessIdentifier = "jmx"
	ActiveMQProcessIdentifier           ProcessIdentifier = "activemq"
	MemcachedProcessIdentifier          ProcessIdentifier = "memcached"
	HaproxyProcessIdentifier            ProcessIdentifier = "haproxy"
	DockerdProcessIdentifier            ProcessIdentifier = "dockerd"
	DockerDesktopJavaProcessIdentifier  ProcessIdentifier = "com.docker.backend"
	SqlservrProcessIdentifier           ProcessIdentifier = "sqlservr"
	// Java Process Identifiers
	JavaProcessIdentifier          ProcessIdentifier = "java"
	CassandraJavaProcessIdentifier ProcessIdentifier = "org.apache.cassandra.service.CassandraDaemon"
	JmxJavaProcessIdentifier       ProcessIdentifier = "com.sun.management.jmxremote"
	ActiveMQJavaProcessIdentifier  ProcessIdentifier = "activemq.jar"
)

var sumoAppProcesses = map[ProcessIdentifier]SumoTag{
	ApacheProcessIdentifier:             ApacheTag,
	Apache2ProcessIdentifier:            ApacheTag,
	HttpdProcessIdentifier:              ApacheTag,
	DockerProcessIdentifier:             DockerTag, // docker cli
	ElasticsearchProcessIdentifier:      ElasticsearchTag,
	MysqlServerProcessIdentifier:        MysqlTag,
	MysqldProcessIdentifier:             MysqlTag,
	NginxProcessIdentifier:              NginxTag,
	PostgresProcessIdentifier:           PostgresTag,
	Postgres95ProcessIdentifier:         PostgresTag,
	RabbitmqServerProcessIdentifier:     RabbitmqTag,
	RedisProcessIdentifier:              RedisTag,
	TomcatProcessIdentifier:             TomcatTag,
	KafkaServerStartShProcessIdentifier: KafkaTag, // Need to test this, most common shell wrapper.
	RedisServerProcessIdentifier:        RedisTag,
	MongodProcessIdentifier:             MongoDBTag,
	CassandraProcessIdentifier:          CassandraTag,
	JmxProcessIdentifier:                JmxTag,
	ActiveMQProcessIdentifier:           ActiveMQTag,
	MemcachedProcessIdentifier:          MemcachedTag,
	HaproxyProcessIdentifier:            HaproxyTag,
	DockerdProcessIdentifier:            DockerCETag, // docker engine, for when process runs natively
	DockerDesktopJavaProcessIdentifier:  DockerCETag, // docker daemon runs on a VM in Docker Desktop, process doesn't show on mac
	SqlservrProcessIdentifier:           MssqlTag,    // linux SQL Server process
	// Java Process Tags
	CassandraJavaProcessIdentifier: CassandraTag,
	JmxJavaProcessIdentifier:       JmxTag,
	ActiveMQJavaProcessIdentifier:  ActiveMQTag,
}

func GetSumoTag(processName string) (SumoTag, bool) {
	tag, ok := sumoAppProcesses[ProcessIdentifier(processName)]
	return tag, ok
}

func (procx *Procx) FilteredProcessList() ([]string, error) {
	var pl []string

	processes, err := procx.getProcesses()
	if err != nil {
		return pl, fmt.Errorf("process discovery failed: %w", err)
	}

	for _, v := range processes {
		processName, ok := procx.getProcessName(v)
		if !ok {
			continue
		}

		if a, i := GetSumoTag(processName); i {
			pl = append(pl, a.String())
		}

		// handling Java background processes
		javaProcessName, ok := procx.getJavaProcessName(processName, v)
		if a, i := GetSumoTag(javaProcessName); i && ok {
			pl = append(pl, a.String())
		}
	}

	return pl, nil
}

func (procx *Procx) getProcessName(process Process) (string, bool) {
	e, err := process.Name()
	if err != nil {
		// If we can't get a process name, it may be a zombie process.
		// We do not want to error out here, as it's not worth disrupting
		// the startup process of the collector.
		procx.logger.Warn(
			"process discovery: failed to get executable name (is it a zombie?)",
			zap.Int32("pid", process.Pid()),
			zap.Error(err))
		return "", false
	}
	return strings.ToLower(e), true
}

func (procx *Procx) getJavaProcessName(processName string, process Process) (string, bool) {
	if processName != JavaProcessIdentifier.String() {
		return "", false
	}

	cmdline, err := process.Cmdline()
	if err != nil {
		procx.logger.Warn(
			"process discovery: failed to get process arguments",
			zap.Int32("pid", process.Pid()),
			zap.Error(err))
		return "", false
	}

	switch {
	case strings.Contains(cmdline, CassandraJavaProcessIdentifier.String()):
		return CassandraJavaProcessIdentifier.String(), true
	case strings.Contains(cmdline, JmxJavaProcessIdentifier.String()):
		return JmxJavaProcessIdentifier.String(), true
	case strings.Contains(cmdline, ActiveMQJavaProcessIdentifier.String()):
		return ActiveMQJavaProcessIdentifier.String(), true
	}
	return "", false
}
