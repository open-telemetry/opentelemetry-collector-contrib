# OpAMP Supervisor: Vision and Roadmap

Status: v1

The OpAMP Supervisor is a reliable control plane runtime for data collection Agents.

It enables users to remotely manage large fleets of Agents with operations such as configuration updates, restarts and upgrades. It transforms a set of Agents into a managed, observable fleet while preserving local autonomy and safety. 

## Agent Scope

[OpenTelemetry Collector](https://opentelemetry.io/docs/collector/)

## Desired Outcomes

### The Watchdog 

The Supervisor monitors the Collector process and restarts it promptly when the process exits unexpectedly, such as after an OOM kill. This reduces telemetry gaps without requiring separate process-management automation for basic recovery.

### Remote Configuration

The Supervisor accepts configuration updates from an OpAMP Server, validates them before applying when validation is enabled, and reloads Collectors as safely as possible. This lets operators manage configuration across large Agent fleets without relying on separate automation tooling such as Ansible or Puppet.

### Centralized Visibility

The Supervisor reports Agent health, version, OS details, effective configuration, and its own telemetry. This gives operators a current view of the deployed fleet and helps them troubleshoot pipeline and configuration problems.

### Agent Lifecycle Management

The Supervisor downloads, verifies, and applies Collector package updates when instructed by an OpAMP Server, with rollback if the updated Collector fails to start. This enables controlled fleet updates without introducing avoidable outage risk.

## Out of Scope

### User Interface 

The Supervisor is not required to expose any UI or dashboards. Any visual management control plane is expected to be provided by OpAMP Server implementations.

### Telemetry Processing

The Supervisor does not touch or alter the telemetry data that the Collector collects and processes. Its job is managing the agent’s configuration and lifecycle - all tracing / metrics / logging pipelines remain defined in the Collector configuration.

### Orchestration

The Supervisor will not perform a higher level coordination or grouping of agents via policies or templates for operations such as configuration, upgrades. This will be handled by the OpAMP Server or other orchestration tools such as Ansible, Chef.

### Configuration Merging / Generation

The configuration file merging rules match the rules already in place in the Collector. The Supervisor is not expected to include any logic or implementation to create or merge configuration.

## Guiding Principles

### Security

The Supervisor must be secure by default because it can change Collector configuration, restart processes, and apply package updates. Communications with OpAMP Servers should use explicit authentication and TLS configuration, downloaded artifacts must be verified before use, and sensitive data must not be exposed through logs, status reports, or telemetry.

### Reliability and Safety

The Supervisor needs to be highly stable, so we need to keep its complexity and functionality to a minimum. It should never compromise the Collector’s uptime or the telemetry data flow. Changes must be applied safely - for example, if the new config is invalid or causes the Collector to crash, the Supervisor should detect this and revert to the last good state (gracefully restarting the Collector with the previous config). Similarly, for upgrades, it should verify package integrity (e.g. checksum or signature) and support rollback if a new binary fails.

### Standardization

All management capabilities (config distribution, status/health reporting, package updates, etc.) will follow the vendor-neutral spec so that any OpAMP-compatible server can work with the Supervisor.

### Self Observability

The Supervisor is a critical component that itself must be observable. It must expose its own health metrics and logs so that users can monitor and troubleshoot its operations (eg. its own resource usage, during remote configuration or validate if an update failed).

### Ease of Use

The Supervisor should be simple to deploy and use - requiring minimal configuration itself to connect to an OpAMP Server and perform supported operations. The documentation to get started and use at scale should be easy to follow.

### Pluggability / Extensibility

Similar to the Collector, the core Supervisor should implement only the minimal, standardized set of OpAMP behaviors. All non-essential or environment-specific functionality must be added through well-defined extension points that are optional, isolated, and independently versioned (eg. contrib).

## Goals

Release a product ready MVP Supervisor 1.0

* Implement the MVP features
* Harden the implementation
* Make official deb/rpm/etc release, bundled with Collector
