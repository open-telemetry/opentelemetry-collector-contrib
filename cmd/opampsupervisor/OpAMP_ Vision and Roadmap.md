# OpAMP Supervisor: Vision and Roadmap

Status: v1

The OpAMP supervisor is a reliable control plane runtime for data collection Agents.  

It enables users to remotely manage large fleets of Agents with operations such as configuration updates, restarts and upgrades. It transforms a set of Agents into a managed, observable fleet while preserving local autonomy and safety. 

## Agent Scope

[OpenTelemetry Collector](https://opentelemetry.io/docs/collector/)

## Desired Outcomes

### The Watchdog 

* When the Collector process crashes due to a memory leak,  
* I want the Supervisor to detect the exit and restart the process immediately,   
* So that I don't get paged at 3 AM for a failure and there is no major telemetry gap 

### Remote Configuration

* When I need to change configuration of the tail sampling processor for 1,000s of OpenTelemetry Collector Agents,   
* I want to push a configuration update from the OpAMP backend that the Supervisor statically validates and reloads the Agents as safely as possible,   
* So that I avoid managing multiple agents via automation tooling such as Ansible, Puppet or manually with incorrect configuration

### Centralized Visibility

* To be able to track the health, version of the fleet of OpenTelemetry Collectors and itself  
* I want the Supervisor to report the agent description including agent version, OS details and the live configuration running at that moment as well as its (Supervisor’s) own telemetry  
* So that I have 100% certainty of the agents running in my infrastructure, understand the status of the supervisor and agents as well as be able to validate agent pipelines, troubleshoot misconfigurations faster

### Agent Lifecycle Management

* When a new version of the OpenTelemetry Collector is released with a critical vulnerability fix,  
* I want the Supervisor to download, verify, and perform the binary update \- but revert immediately if it fails to start,   
* So that I can deploy security patches to production without risking a fleet-wide outage 

## Out of Scope

### User Interface 

The Supervisor is not required to expose any UI or dashboards. Any visual management control plane is expected to be provided by the OpAMP Server implementations. 

### Telemetry Processing

The Supervisor does not touch or alter the telemetry data that the Collector collects and processes. Its job is managing the agent’s configuration and lifecycle \- all tracing / metrics / logging pipelines remain defined in the Collector configuration.

### Orchestration

The Supervisor will not perform a higher level coordination or grouping of agents via policies or templates for operations such as configuration, upgrades. This will be handled by the OpAMP Server or other orchestration tools such as Ansible, Chef. 

### Configuration Merging / Generation

The configuration file merging rules match the rules already in place in the Collector. The Supervisor is not expected to include any logic or implementation to create or merge configuration.

## Guiding Principles

### Reliability and Safety

The Supervisor needs to be highly stable, so we need to keep its complexity and functionality to a minimum. It should never compromise the Collector’s uptime or the telemetry data flow. Changes must be applied safely \- for example, if the new config is invalid or causes the Collector to crash, the Supervisor should detect this and revert to the last good state (gracefully restarting the Collector with the previous config). Similarly, for upgrades, it should verify package integrity (e.g. checksum or signature) and support rollback if a new binary fails. 

### Standardization

All management capabilities (config distribution, status/health reporting, package updates, etc.) will follow the vendor-neutral spec so that any OpAMP-compatible server can work with the Supervisor.

### Self Observability

The Supervisor is a critical component that itself must be observable. It must expose its own health metrics and logs so that users can monitor and troubleshoot its operations (eg. its own resource usage, during remote configuration or validate if an update failed).

### Ease of Use

The Supervisor should be simple to deploy and use \- requiring minimal configuration itself to connect to an OpAMP server and perform supported operations. The documentation to get started and use at scale should be easy to follow.

### Pluggability / Extensibility

Similar to the Collector, the core Supervisor should implement only the minimal, standardized set of OpAMP behaviors. All non-essential or environment-specific functionality must be added through well-defined extension points that are optional, isolated, and independently versioned (eg. contrib).

## Goals

Release a product ready MVP Supervisor 1.0

* Implement the MVP features (to be decided)  
* Harden the implementation  
* Make official deb/rpm/etc release, bundled with Collector