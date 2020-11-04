# Examples
## Kubernetes configuration

### Helm chart values template
[kubernetes/custom-values.yaml](./kubernetes/custom-values.yaml) contains 
an example template for Sumologic Kubernetes Collection Helm chart, which
installs OpenTelemetry Collector in Agent and Gateway configuration, as described
in the [documentation](https://help.sumologic.com/Traces/Getting_Started_with_Transaction_Tracing/Set_up_traces_collection_for_Kubernetes_environments).

After filling the template values, you can install it following
[Sumologic Kubernetes Collection installation instructions](https://github.com/SumoLogic/sumologic-kubernetes-collection/blob/release-v2.0/deploy/docs/Installation_with_Helm.md)
For example, by running following commands:
```shell
helm repo add sumologic https://sumologic.github.io/sumologic-kubernetes-collection
kubectl create namespace sumologic
helm upgrade --install my-release -n sumologic sumologic/sumologic -f custom-values.yaml 
```

### Helm chart values template with cascading filter enabled

Additionally, [kubernetes/custom-values-cascading-filter.yaml](./kubernetes/custom-values-cascading-filter.yaml) 
includes an alternative example template that enables cascading filter,
as described in [trace filtering documentation](https://help.sumologic.com/Traces/Getting_Started_with_Transaction_Tracing/What_if_I_don't_want_to_send_all_the_tracing_data_to_Sumo_Logic%3F).
Note that cascading filter is currently supported only for single-instance
OpenTelemetry Collector deployments.

## Non-kubernetes configuration

### Agent configuration (should be run on each host/node)
[non-kubernetes/agent-configuration-template.yaml](non-kubernetes/agent-configuration-template.yaml) contains
an OpenTelemetry Collector YAML file which includes configuration
for OpenTelemetry Collector running in Agent mode. It should be 
deployed on each host/node within the system.

### Gateway configuration (should be run per each cluster/data-center/etc.)
[non-kubernetes/gateway-configuration-template.yaml](non-kubernetes/gateway-configuration-template.yaml) contains
an OpenTelemetry Collector YAML file which includes configuration
for OpenTelemetry Collector running in Gateway mode. 

Additionally, for [non-kubernetes/gateway-configuration-template-with-cascading-filter.yaml](non-kubernetes/gateway-configuration-template-with-cascading-filter.yaml)
the configuration also includes cascading filter config,
which is described in more detail in [trace filtering documentation](https://help.sumologic.com/Traces/Getting_Started_with_Transaction_Tracing/What_if_I_don't_want_to_send_all_the_tracing_data_to_Sumo_Logic%3F).

Please refer to [relevant documentation](https://help.sumologic.com/Traces/Getting_Started_with_Transaction_Tracing/Set_up_traces_collection_for_other_environments)
for more details.

### AWS OTel Collector configuration file
[non-kubernetes/aws-otel-config.yaml](non-kubernetes/aws-otel-config.yaml) contains
an AWS OpenTelemetry Collector distrubtion YAML file which includes configuration
for OpenTelemetry Collector. Should be deployed on the AWS environments.

### AWS OTel Collector for ECS in EC2 mode template
[non-kubernetes/aws-otel-ecs-ec2-deployment.yaml](non-kubernetes/aws-otel-ecs-ec2-deployment.yaml) contains
an AWS OpenTelemetry Collector distribution YAML file which includes  
CloudFormation template. It should be deployed on the AWS ECS EC2  
environment.

### AWS OTel Collector for ECS in Fargate mode template
[non-kubernetes/aws-otel-ecs-fargate-deployment.yaml](non-kubernetes/aws-otel-ecs-fargate-deployment.yaml) contains
an AWS OpenTelemetry Collector distribution YAML file which includes  
CloudFormation template. It should be deployed on the AWS ECS Fargate  
environment.

### AWS OTel Collector for EC2 deployment template
[non-kubernetes/aws-otel-ec2-deployment.yaml](non-kubernetes/aws-otel-ec2-deployment.yaml) contains
an AWS OpenTelemetry Collector distribution YAML file which includes  
CloudFormation template. It should be deployed on the AWS EC2.

### AWS Distro for OpenTelemetry configuration
[aws_lambda/aws-distro-collector-lambda-layer-config.yaml](aws_lambda/aws-distro-collector-lambda-layer-config.yaml) contains 
an [AWS Distro for Opentelemetry Collector](https://github.com/aws-observability/aws-otel-lambda/tree/main/extensions/aoc-extension) YAML file which includes configuration for collector installed in a Lambda Layer. Collector requires *SUMOLOGIC_HTTP_TRACES_ENDPOINT_URL* environment variable to be set. 
