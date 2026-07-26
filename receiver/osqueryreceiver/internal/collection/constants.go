// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package collection // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/osqueryreceiver/internal/collection"

// singletonRowKey is the RowKey for collections whose query returns at most
// one row, where diffing degenerates to "did the one row change".
const singletonRowKey = "singleton"

const (
	// system_info
	systemInfoCollectionName  = "system_info"
	systemInfoCollectionQuery = `SELECT hostname, uuid, cpu_type, cpu_subtype, cpu_brand, cpu_physical_cores, cpu_logical_cores, physical_memory, hardware_vendor, hardware_model
FROM system_info;`

	// package_info
	packageInfoCollectionName          = "package_info"
	packageInfoCollectionQueryHomebrew = `SELECT * from homebrew_packages;`
	packageInfoCollectionQueryDebian   = `SELECT * from deb_packages;`
	packageInfoCollectionQueryRPM      = `SELECT * from rpm_packages;`

	// os_info
	osInfoCollectionName  = "os_info"
	osInfoCollectionQuery = `SELECT * FROM os_version;`

	// secureboot
	secureBootCollectionName  = "secureboot_info"
	secureBootCollectionQuery = `SELECT * FROM secureboot;`

	// users
	userCollectionName       = "users_info"
	userCollectionQueryLinux = `SELECT u.username, GROUP_CONCAT(g.groupname, ', ') AS groups
FROM users u
JOIN user_groups ug ON u.uid = ug.uid
JOIN groups g ON ug.gid = g.gid
WHERE
    u.uid >= 1000
    AND u.uid != 65534 -- Exclude 'nobody'
    AND u.shell NOT IN ('/usr/sbin/nologin', '/bin/false')
GROUP BY u.username;`
	userCollectionQueryDarwin = `SELECT u.username, GROUP_CONCAT(g.groupname, ', ') AS groups
FROM users u
JOIN user_groups ug ON u.uid = ug.uid
JOIN groups g ON ug.gid = g.gid
WHERE
    u.uid >= 500
    AND u.uid != 65534 -- Exclude 'nobody'
    AND u.shell NOT IN ('/usr/sbin/nologin', '/bin/false')
GROUP BY u.username;`
	userCollectionQueryWindows = `SELECT u.username, GROUP_CONCAT(g.groupname, ', ') AS groups
FROM users u
JOIN user_groups ug ON u.uid = ug.uid
JOIN groups g ON ug.gid = g.gid
WHERE
    u.directory LIKE 'C:\\Users\\%'
    AND
    u.username NOT IN ('Administrator', 'Guest', 'SYSTEM', 'LOCAL SERVICE', 'NETWORK SERVICE')
GROUP BY u.username;`
)
