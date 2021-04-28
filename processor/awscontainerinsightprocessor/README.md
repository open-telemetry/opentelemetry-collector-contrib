# AWS Container Insights Processor

## Overview
AWS Container Insights Processor (`awscontainerinsightprocessor`) is an AWS specific processor that supports [CloudWatch Container Insights]((https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/ContainerInsights.html)). It decorates the metrics emitted by `awscontainerinsightreceiver` to support the same CloudWatch Container Insights experience. Unlike [kubeletstatsreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/kubeletstatsreceiver) and [k8sprocessor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/k8sprocessor) which can work indepedently, `awscontainerinsightprocessor` is supposed to be used togther with `awscontainerinsightreceiver` and it expects certain metrics and resource attributes to be present in order to do the decoration (see [readme for `awscontainerinsightreceiver`](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/receiver/awscontainerinsightreceiver/README.md)). 

## Design of AWS Container Insights Processor for EKS

* pod store:
  * access Kubelet to list all Pod objects running on the same node, and cache all the Pod objects in memory
  * find corresponding pod objects from the cache to decorate pod and container metrics
  * information used to decorate pod and container metrics:
    1. Pod Owner Reference
    2. Container Resource Limit
    3. Container Resource Request
    4. Container ID and Status
    5. Pod Status
    6. Pod Labels
* service store: 
  * add the corresponding attribute `Service` to the metrics for the relevant pods
  * call "List & Watch" for "Endpoint" objects from Kubernetes API server. In this mode, when there is any change to “Endpoint”, agent will get the notification of the change. The benefit is that the agent could get Endpoint information update in time. If the "Endpoint" resources just occasionally slightly changes, the impact to API server is supposed to be minimal.
  * If customer is not interested in aggregating metrics per ServiceName, we allow them to disable the service decoration through the configuration.


## Available Metrics and Resource Attributes

The AWS Container Insights Processor decorates metrics differently depending on their types.

### Cluster metrics
No decoration.

### Cluster Namespace metrics
No decoration.

### Cluster Service metrics
No decoration.


### Node metrics
| Added Metric                      | Unit      |
|-----------------------------------|-----------|
| node_cpu_request                  | Millicore |
| node_cpu_reserved_capacity        | Percent   |
| node_memory_request               | Bytes     |
| node_memory_reserved_capacity     | Percent   |
| node_number_of_running_containers | Count     |
| node_number_of_running_pods       | Count     |

The following resource attributes are added:
* Sources
* kubernetes 


### Node Disk IO metrics
The following resource attributes are added:
* Sources
* kubernetes



### Node Filesystem metrics
The following resource attributes are added:
* Sources
* kubernetes


### Node Network metrics
The following resource attributes are added:
* Sources
* kubernetes


### Pod metrics
| Added Metric                          | Unit      | 
|---------------------------------------|-----------|
| pod_cpu_limit                         | Millicore |
| pod_cpu_request                       | Millicore |
| pod_cpu_reserved_capacity             | Percent   |
| pod_cpu_utilization_over_pod_limit    | Percent   |
| pod_memory_limit                      | Bytes     |
| pod_memory_request                    | Bytes     |
| pod_memory_reserved_capacity          | Percent   |
| pod_memory_utilization_over_pod_limit | Percent   |
| pod_number_of_container_restarts      | Count     | 
| pod_number_of_containers              | Count     |   
| pod_number_of_running_containers      | Count     |  

The following resource attributes are added:
* Sources
* kubernetes
* pod_status



### Pod Network metrics
The following resource attributes are added:
* Sources
* kubernetes
* pod_status


### Container metrics
| Added Metric                 | Unit      |
|------------------------------|-----------|
| container_cpu_limit          | Millicore |
| container_cpu_request        | Millicore |
| container_memory_limit       | Bytes     |
| container_memory_request     | Bytes     |
| number_of_container_restarts | Count     |


The following resource attributes are added:
* Sources
* kubernetes
* container_status
* container_status_reason
* container_last_termination_reason 

The attribute `container_status_reason` is present only when `container_status` is in "Waiting" or "Terminated" State. The attribute `container_last_termination_reason` is present only when `container_status` is in "Terminated" State.


**Notes on some attributes**
The `Sources` attribute looks like this:
```
"Sources": [
        "cadvisor",
        "pod",
        "calculated"
    ]
```

The `kubernetes` attribute looks like this:
* for node-level metrics
```
"kubernetes": {
        "host": "ip-192-168-12-170.ec2.internal"
    }
```

* for pod level metrics
```
"kubernetes": {
        "host": "ip-192-168-12-170.ec2.internal",
        "labels": {
            "controller-revision-hash": "69bf46b494",
            "k8s-app": "aws-node",
            "pod-template-generation": "1"
        },
        "namespace_name": "kube-system",
        "pod_id": "33f134f1-73c5-4884-9e6e-12f0a30ecba3",
        "pod_name": "aws-node-pqs9w",
        "pod_owners": [
            {
                "owner_kind": "DaemonSet",
                "owner_name": "aws-node"
            }
        ]
    }
```

* for pod and container level metrics
```
"kubernete": {
    "container_name": "kube-state-metrics",
    "docker": {
        "container_id": "e09f2d41d1dd03a40290912cac2b1a0948fb6d0243b2fb2381a1b77bcd8576d0"
    },
    "host": "ip-192-168-12-170.ec2.internal",
    "labels": {
        "app.kubernetes.io/name": "kube-state-metrics",
        "app.kubernetes.io/version": "2.0.0-beta",
        "pod-template-hash": "5b9d68779b"
    },
    "namespace_name": "kube-system",
    "pod_id": "09d57585-9172-4c54-894e-3d794587a9f5",
    "pod_name": "kube-state-metrics-5b9d68779b-qqjb9",
    "pod_owners": [
        {
            "owner_kind": "Deployment",
            "owner_name": "kube-state-metrics"
        }
    ],
    "service_name": "kube-state-metrics"
}
```

## Sample deployment yaml file
The following sample yaml file can be used to deploy 
```
# create namespace
apiVersion: v1
kind: Namespace
metadata:
  name: aws-otel-eks
  labels:
    name: aws-otel-eks

---
# create cwagent service account and role binding
apiVersion: v1
kind: ServiceAccount
metadata:
  name: aws-otel-sa
  namespace: aws-otel-eks

---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: aoc-agent-role
rules:
  - apiGroups: [""]
    resources: ["pods", "nodes", "endpoints"]
    verbs: ["list", "watch"]
  - apiGroups: ["apps"]
    resources: ["replicasets"]
    verbs: ["list", "watch"]
  - apiGroups: ["batch"]
    resources: ["jobs"]
    verbs: ["list", "watch"]
  - apiGroups: [""]
    resources: ["nodes/proxy"]
    verbs: ["get"]
  - apiGroups: [""]
    resources: ["nodes/stats", "configmaps", "events"]
    verbs: ["create", "get"]
  - apiGroups: [""]
    resources: ["configmaps"]
    resourceNames: ["aoc-clusterleader"]
    verbs: ["get","update"]

---
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: aoc-agent-role-binding
subjects:
  - kind: ServiceAccount
    name: aws-otel-sa
    namespace: aws-otel-eks
roleRef:
  kind: ClusterRole
  name: aoc-agent-role
  apiGroup: rbac.authorization.k8s.io

---
apiVersion: v1
kind: ConfigMap
metadata:
  name: otel-agent-conf
  namespace: aws-otel-eks
  labels:
    app: opentelemetry
    component: otel-agent-conf
data:
  otel-agent-config: |
    extensions:
      health_check:

    receivers:
      awscontainerinsightreceiver:

    processors:
      awscontainerinsightprocessor:

      batch/metrics:
        timeout: 60s

    exporters:
      awsemf:
        namespace: ContainerInsights
        log_group_name: '/aws/containerinsights/{ClusterName}/performance'
        log_stream_name: '{NodeName}'
        resource_to_telemetry_conversion:
          enabled: true
        dimension_rollup_option: NoDimensionRollup
        parse_json_encoded_attr_values: [Sources, kubernetes]
        metric_declarations:
          # node metrics
          - dimensions: [[NodeName, InstanceId, ClusterName]]
            metric_name_selectors:
              - node_cpu_utilization
              - node_memory_utilization
              - node_network_total_bytes
              - node_cpu_reserved_capacity
              - node_memory_reserved_capacity
              - node_number_of_running_pods
              - node_number_of_running_containers
          - dimensions: [[ClusterName]]
            metric_name_selectors:
              - node_cpu_utilization
              - node_memory_utilization
              - node_network_total_bytes
              - node_cpu_reserved_capacity
              - node_memory_reserved_capacity
              - node_number_of_running_pods
              - node_number_of_running_containers
              - node_cpu_usage_total
              - node_cpu_limit
              - node_memory_working_set
              - node_memory_limit

          # pod metrics
          - dimensions: [[PodName, Namespace, ClusterName], [Service, Namespace, ClusterName], [Namespace, ClusterName], [ClusterName]]
            metric_name_selectors:
              - pod_cpu_utilization
              - pod_memory_utilization
              - pod_network_rx_bytes
              - pod_network_tx_bytes
              - pod_cpu_utilization_over_pod_limit
              - pod_memory_utilization_over_pod_limit
          - dimensions: [[PodName, Namespace, ClusterName], [ClusterName]]
            metric_name_selectors:
              - pod_cpu_reserved_capacity
              - pod_memory_reserved_capacity
          - dimensions: [[PodName, Namespace, ClusterName]]
            metric_name_selectors:
              - pod_number_of_container_restarts

          # cluster metrics
          - dimensions: [[ClusterName]]
            metric_name_selectors:
              - cluster_node_count
              - cluster_failed_node_count

          # service metrics
          - dimensions: [[Service, Namespace, ClusterName], [ClusterName]]
            metric_name_selectors:
              - service_number_of_running_pods

          # node fs metrics
          - dimensions: [[NodeName, InstanceId, ClusterName], [ClusterName]]
            metric_name_selectors:
              - node_filesystem_utilization

          # namespace metrics
          - dimensions: [[Namespace, ClusterName], [ClusterName]]
            metric_name_selectors:
              - namespace_number_of_running_pods


      logging:
        loglevel: debug

    service:
      pipelines:
        metrics:
          receivers: [awscontainerinsightreceiver]
          processors: [awscontainerinsightprocessor, batch/metrics]
          exporters: [awsemf]

      extensions: [health_check]


---
# create Daemonset
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: aws-otel-eks-ci
  namespace: aws-otel-eks
spec:
  selector:
    matchLabels:
      name: aws-otel-eks-ci
  template:
    metadata:
      labels:
        name: aws-otel-eks-ci
    spec:
      containers:
        - name: aws-otel-collector
          image: amazon/aws-otel-collector:latest
          env:
            - name: AWS_REGION
              value: "us-east-1"
            - name: K8S_NODE_NAME
              valueFrom:
                fieldRef:
                  fieldPath: spec.nodeName
            - name: HOST_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.hostIP
            - name: HOST_NAME
              valueFrom:
                fieldRef:
                  fieldPath: spec.nodeName
            - name: K8S_NAMESPACE
              valueFrom:
                 fieldRef:
                   fieldPath: metadata.namespace
          imagePullPolicy: Always
          command:
            - "/awscollector"
              #- "--log-level=DEBUG"
            - "--config=/conf/otel-agent-config.yaml"
          volumeMounts:
            - name: rootfs
              mountPath: /rootfs
              readOnly: true
            - name: dockersock
              mountPath: /var/run/docker.sock
              readOnly: true
            - name: varlibdocker
              mountPath: /var/lib/docker
              readOnly: true
            - name: sys
              mountPath: /sys
              readOnly: true
            - name: devdisk
              mountPath: /dev/disk
              readOnly: true
            - name: otel-agent-config-vol
              mountPath: /conf
          resources:
            limits:
              cpu:  200m
              memory: 200Mi
            requests:
              cpu: 200m
              memory: 200Mi
      volumes:
        - configMap:
            name: otel-agent-conf
            items:
              - key: otel-agent-config
                path: otel-agent-config.yaml
          name: otel-agent-config-vol
        - name: rootfs
          hostPath:
            path: /
        - name: dockersock
          hostPath:
            path: /var/run/docker.sock
        - name: varlibdocker
          hostPath:
            path: /var/lib/docker
        - name: sys
          hostPath:
            path: /sys
        - name: devdisk
          hostPath:
            path: /dev/disk/
      serviceAccountName: aws-otel-sa
```