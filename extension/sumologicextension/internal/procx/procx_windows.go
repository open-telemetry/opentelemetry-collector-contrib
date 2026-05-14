// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package procx // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/sumologicextension/internal/procx"

import (
	"fmt"
	"strings"

	"go.uber.org/zap"
)

const (
	ApacheProcessIdentifier   ProcessIdentifier = "apache.exe"
	HttpdProcessIdentifier    ProcessIdentifier = "httpd.exe"
	DockerProcessIdentifier   ProcessIdentifier = "docker.exe"
	MysqldProcessIdentifier   ProcessIdentifier = "mysqld.exe"
	NginxProcessIdentifier    ProcessIdentifier = "nginx.exe"
	PostgresProcessIdentifier ProcessIdentifier = "postgresql.exe"
	TomcatProcessIdentifier   ProcessIdentifier = "tomcat.exe"
	Tomcat9ProcessIdentifier  ProcessIdentifier = "tomcat9.exe"
	MongodProcessIdentifier   ProcessIdentifier = "mongod.exe"
	DockerdProcessIdentifier  ProcessIdentifier = "dockerd.exe"
	SqlservrProcessIdentifier ProcessIdentifier = "sqlservr.exe"
	// Java Process Identifiers
	JavaProcessIdentifier          ProcessIdentifier = "java.exe"
	ElasticsearchProcessIdentifier ProcessIdentifier = "elasticsearch" // cmdline args does not have exe
	CasandraJavaProcessIdentifier  ProcessIdentifier = "org.apache.cassandra.service.CassandraDaemon"
	JmxJavaProcessIdentifier       ProcessIdentifier = "com.sun.management.jmxremote"
	ActiveMQJavaProcessIdentifier  ProcessIdentifier = "activemq.jar"
	// Erlang Process Identifiers
	ErlangProcessIdentifier         ProcessIdentifier = "erl.exe"
	RabbitmqServerProcessIdentifier ProcessIdentifier = "rabbit"
)

var sumoAppProcesses = map[ProcessIdentifier]SumoTag{
	ApacheProcessIdentifier:   ApacheTag,
	HttpdProcessIdentifier:    ApacheTag,
	DockerProcessIdentifier:   DockerTag, // docker cli
	MysqldProcessIdentifier:   MysqlTag,
	NginxProcessIdentifier:    NginxTag,
	PostgresProcessIdentifier: PostgresTag,
	TomcatProcessIdentifier:   TomcatTag,
	Tomcat9ProcessIdentifier:  TomcatTag,
	MongodProcessIdentifier:   MongoDBTag,
	DockerdProcessIdentifier:  DockerCETag, // docker engine, for when process runs natively
	SqlservrProcessIdentifier: MssqlTag,
	// Java Process Tags
	ElasticsearchProcessIdentifier: ElasticsearchTag,
	CasandraJavaProcessIdentifier:  CassandraTag,
	JmxJavaProcessIdentifier:       JmxTag,
	ActiveMQJavaProcessIdentifier:  ActiveMQTag,
	// Erlang Process Identifiers
	RabbitmqServerProcessIdentifier: RabbitmqTag,
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

		// handling erlang processes
		erlProcessName, ok := procx.getErlangProcessName(processName, v)
		if a, i := GetSumoTag(erlProcessName); i && ok {
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
	case strings.Contains(cmdline, ElasticsearchProcessIdentifier.String()):
		return ElasticsearchProcessIdentifier.String(), true
	case strings.Contains(cmdline, CasandraJavaProcessIdentifier.String()):
		return CasandraJavaProcessIdentifier.String(), true
	case strings.Contains(cmdline, JmxJavaProcessIdentifier.String()):
		return JmxJavaProcessIdentifier.String(), true
	case strings.Contains(cmdline, ActiveMQJavaProcessIdentifier.String()):
		return ActiveMQJavaProcessIdentifier.String(), true
	}
	return "", false
}

func (procx *Procx) getErlangProcessName(processName string, process Process) (string, bool) {
	if processName != ErlangProcessIdentifier.String() {
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

	if strings.Contains(cmdline, RabbitmqServerProcessIdentifier.String()) {
		return RabbitmqServerProcessIdentifier.String(), true
	}
	return "", false
}
