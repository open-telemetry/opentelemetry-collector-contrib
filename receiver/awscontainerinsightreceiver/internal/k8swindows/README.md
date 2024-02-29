## Available Metrics and Resource Attributes for Windows worker Nodes

### Node
| Metric                                    | Unit         |
|-------------------------------------------|--------------|
| node_cpu_limit                            | Millicore    |
| node_cpu_request                          | Millicore    |
| node_cpu_reserved_capacity                | Percent      |
| node_cpu_usage_total                      | Millicore    |
| node_cpu_utilization                      | Percent      |
| node_memory_limit                         | Bytes        |
| node_memory_pgfault                       | Count/Second |
| node_memory_pgmajfault                    | Count/Second |
| node_memory_request                       | Bytes        |
| node_memory_reserved_capacity             | Percent      |
| node_memory_rss                           | Bytes        |
| node_memory_usage                         | Bytes        |
| node_memory_utilization                   | Percent      |
| node_memory_working_set                   | Bytes        |
| node_network_rx_bytes                     | Bytes/Second |
| node_network_rx_dropped                   | Count/Second |
| node_network_rx_errors                    | Count/Second |
| node_network_total_bytes                  | Bytes/Second |
| node_network_tx_bytes                     | Bytes/Second |
| node_network_tx_dropped                   | Count/Second |
| node_network_tx_errors                    | Count/Second |
| node_number_of_running_containers         | Count        |
| node_number_of_running_pods               | Count        |
| node_status_condition_ready               | Count        |
| node_status_condition_pid_pressure        | Count        |
| node_status_condition_memory_pressure     | Count        |
| node_status_condition_disk_pressure       | Count        |
| node_status_condition_network_unavailable | Count        |
| node_status_condition_unknown             | Count        |
| node_status_capacity_pods                 | Count        |
| node_status_allocatable_pods              | Count        |

<br/><br/>
| Resource Attribute    |
|-----------------------|
| ClusterName           |
| InstanceType          |
| NodeName              |
| Timestamp             |
| Type                  |
| Version               |
| Sources               |
| kubernetes            |
| OperatingSystem       |

<br/><br/>
<br/><br/>

### Node Filesystem
| Metric                       | Unit    |
|------------------------------|---------|
| node_filesystem_available    | Bytes   |
| node_filesystem_capacity     | Bytes   |
| node_filesystem_usage        | Bytes   |
| node_filesystem_utilization  | Percent |

<br/><br/>
| Resource Attribute    |
|---------------------- |
| AutoScalingGroupName  |
| ClusterName           |
| InstanceId            |
| InstanceType          |
| NodeName              |
| Timestamp             |
| Type                  |
| Version               |
| Sources               |
| kubernete             |
| OperatingSystem       |
<br/><br/>
<br/><br/>

### Node Network
| Metric                             | Unit         |
|------------------------------------|--------------|
| node_interface_network_rx_bytes    | Bytes/Second |
| node_interface_network_rx_dropped  | Count/Second |
| node_interface_network_rx_errors   | Count/Second |
| node_interface_network_total_bytes | Bytes/Second |
| node_interface_network_tx_bytes    | Bytes/Second |
| node_interface_network_tx_dropped  | Count/Second |
| node_interface_network_tx_errors   | Count/Second |

<br/><br/>
| Resource Attribute    |
|-----------------------|
| AutoScalingGroupName  |
| ClusterName           |
| InstanceId            |
| InstanceType          |
| NodeName              |
| Timestamp             |
| Type                  |
| Version               |
| interface             |
| Sources               |
| kubernete             |
| OperatingSystem       |
<br/><br/>
<br/><br/>

### Pod
| Metric                                                            | Unit         |
|-------------------------------------------------------------------|--------------|
| pod_cpu_limit                                                     | Millicore    |
| pod_cpu_request                                                   | Millicore    |
| pod_cpu_reserved_capacity                                         | Percent      |
| pod_cpu_usage_total                                               | Millicore    |
| pod_cpu_utilization                                               | Percent      |
| pod_cpu_utilization_over_pod_limit                                | Percent      |
| pod_memory_limit                                                  | Bytes        |
| pod_memory_max_usage                                              | Bytes        |
| pod_memory_pgfault                                                | Count/Second |
| pod_memory_pgmajfault                                             | Count/Second |
| pod_memory_request                                                | Bytes        |
| pod_memory_reserved_capacity                                      | Percent      |
| pod_memory_rss                                                    | Bytes        |
| pod_memory_usage                                                  | Bytes        |
| pod_memory_utilization                                            | Percent      |
| pod_memory_utilization_over_pod_limit                             | Percent      |
| pod_memory_working_set                                            | Bytes        |
| pod_network_rx_bytes                                              | Bytes/Second |
| pod_network_rx_dropped                                            | Count/Second |
| pod_network_rx_errors                                             | Count/Second |
| pod_network_total_bytes                                           | Bytes/Second |
| pod_network_tx_bytes                                              | Bytes/Second |
| pod_network_tx_dropped                                            | Count/Second |
| pod_network_tx_errors                                             | Count/Second |
| pod_number_of_container_restarts                                  | Count        | 
| pod_number_of_containers                                          | Count        |   
| pod_number_of_running_containers                                  | Count        |  
| pod_status_ready                                                  | Count        |
| pod_status_scheduled                                              | Count        |
| pod_status_unknown                                                | Count        |
| pod_status_failed                                                 | Count        |
| pod_status_pending                                                | Count        |
| pod_status_running                                                | Count        |
| pod_status_succeeded                                              | Count        |
| pod_container_status_running                                      | Count        |
| pod_container_status_terminated                                   | Count        |
| pod_container_status_waiting                                      | Count        |
| pod_container_status_waiting_reason_crash_loop_back_off           | Count        |
| pod_container_status_waiting_reason_image_pull_error              | Count        |
| pod_container_status_waiting_reason_start_error                   | Count        |
| pod_container_status_waiting_reason_create_container_error        | Count        |
| pod_container_status_waiting_reason_create_container_config_error | Count        |
| pod_container_status_terminated_reason_oom_killed                 | Count        |

| Resource Attribute     |
|------------------------|
| AutoScalingGroupName   |
| ClusterName            |
| InstanceId             |
| InstanceType           |
| K8sPodName             |
| Namespace              |
| NodeName               |
| PodId                  |
| Timestamp              |
| Type                   |
| Version                |
| Sources                |
| kubernete              |
| pod_status             |
| OperatingSystem        |

<br/><br/>

### Pod Network
| Metric                            | Unit         |
|-----------------------------------|--------------|
| pod_interface_network_rx_bytes    | Bytes/Second |
| pod_interface_network_rx_dropped  | Count/Second |
| pod_interface_network_rx_errors   | Count/Second |
| pod_interface_network_total_bytes | Bytes/Second |
| pod_interface_network_tx_bytes    | Bytes/Second |
| pod_interface_network_tx_dropped  | Count/Second |
| pod_interface_network_tx_errors   | Count/Second |


<br/><br/>
| Resource Attribute   |
|----------------------|
| AutoScalingGroupName |
| ClusterName          |
| InstanceId           |
| InstanceType         |
| K8sPodName           |
| Namespace            |
| NodeName             |
| PodId                |
| Timestamp            |
| Type                 |
| Version              |
| interface            |
| Sources              |
| kubernete            |
| pod_status           |
| OperatingSystem        |

<br/><br/>
<br/><br/>


### Container
| Metric                                            | Unit         |
|---------------------------------------------------|--------------|
| container_cpu_limit                               | Millicore    |
| container_cpu_request                             | Millicore    |
| container_cpu_usage_total                         | Millicore    |
| container_cpu_utilization                         | Percent      |
| container_cpu_utilization_over_container_limit    | Percent      |
| container_memory_limit                            | Bytes        |
| container_memory_mapped_file                      | Bytes        |
| container_memory_pgfault                          | Count/Second |
| container_memory_pgmajfault                       | Count/Second |
| container_memory_request                          | Bytes        |
| container_memory_rss                              | Bytes        |
| container_memory_usage                            | Bytes        |
| container_memory_utilization                      | Percent      |
| container_memory_utilization_over_container_limit | Percent      |
| container_memory_working_set                      | Bytes        |
| number_of_container_restarts                      | Count        |

<br/><br/>

| Resource Attribute                |
|-----------------------------------|
| AutoScalingGroupName              |
| ClusterName                       |
| ContainerId                       |
| ContainerName                     |
| InstanceId                        |
| InstanceType                      |
| K8sPodName                        |
| Namespace                         |
| NodeName                          |
| PodId                             |
| Timestamp                         |
| Type                              |
| Version                           |
| Sources                           |
| kubernetes                        |
| container_status                  |
| container_status_reason           |
| container_last_termination_reason | 
| OperatingSystem                   |
