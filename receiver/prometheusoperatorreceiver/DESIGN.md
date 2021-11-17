## Design Goals

### Vision
Provide a nearly zero-config migration path from PrometheusOperator hosted scraping config to using the collector.  

### Scope
Only scrape config relevant CRDs (CustomResourceDefinition) are in scope of this receiver.    
As of this writing the relevant CRDs are: 
- ServiceMonitor
- PodMonitor

The receiver should be able to query this CRDs from Kubernetes, attach watchers and reconfigure its scraping config 
based on existing CRs (CustomResource) in Kubernetes.

Changes as well as creations or deletions of said CRs should trigger reconciliation of the scrape config. 
Active querying of the API should only occur during startup and in a defined interval of the receiver. 

Namespace limitations should be possible for shared cluster scenarios.     
This should be implemented using industry standard methods. 


### Out of scope
Excluded are other relevant CRDs such as: 
- PrometheusRules
- ThanosRuler
- Probes
- Alertmanagers
- AlertmanagerConfigs

This includes concepts like alerting and black box probing of defined targets. 


## PrometheusOperator CR watcher
The CR watcher is the component responsible for querying CRs from Kubernetes API and triggering a reconciliation.

### Major components of CR watcher
- **[ListenerFactory](https://github.com/prometheus-operator/prometheus-operator/blob/main/pkg/informers/monitoring.go):**
  the component which creates listeners on the CR
- **[APIClient](https://github.com/prometheus-operator/prometheus-operator/tree/main/pkg/client):**
  PrometheusOperator API client

## Config generator
The config generator is triggered by the watcher and generates a Prometheus config as byte array.   
Instead of writing it onto a disk the configuration is unmarshalled using the Prometheus config loader, 
which is already in use by the `prometheusreceiver` resulting in a full Prometheus config. This config is 
used to create a `prometheusreceiver` `Config` struct.  

### Major components of the config generation
- **[ConfigGenerator](https://github.com/prometheus-operator/prometheus-operator/blob/main/pkg/prometheus/promcfg.go#L304):**
  PrometheusOperator component which generates a valid Prometheus config marshalled to a byte array
- **[ConfigLoader](https://github.com/prometheus/prometheus/blob/main/config/config.go#L68):**
  Prometheus configuration loader which unmarshalls the config to a `prometheusreceiver` usable object 


## Processing config change events
The Prometheus config is at first compared against the current applied one. Should there be any change, a new 
`prometheusreceiver` is started using the generated configuration. 
If the startup is successful the old instance of `prometheusreceiver` is shutdown. 
Should any error occur during startup the old instance will keep running

