// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

db.getSiblingDB('sample_db').createCollection('users');

// Switch to admin db to create users
db.getSiblingDB('admin')
.createUser({
  user: 'app_user',
  pwd: 'app_password',
  roles: [{
    role: 'readWrite',
    db: 'sample_db'
  }]
});

db.getSiblingDB("admin").createUser({
  user: "otel-user",
  pwd: "otel-password",
  roles: [
    { role: "read", db: "admin" },
    { role: "read", db: "local" },
    { role: "clusterMonitor", db: "admin" }
  ]
})

db.getSiblingDB("admin").grantRolesToUser("otel-user", [
  { role: "readAnyDatabase", db: "admin" }
])