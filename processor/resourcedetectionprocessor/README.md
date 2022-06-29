# Resource Detection Processor

| Status                   |                       |
| ------------------------ | --------------------- |
| Stability                | [beta]                |
| Supported pipeline types | traces, metrics, logs |
| Distributions            | [contrib]             |

The resource detection processor can be used to detect resource information from the host,
in a format that conforms to the [OpenTelemetry resource semantic conventions](https://github.com/open-telemetry/opentelemetry-specification/tree/main/specification/resource/semantic_conventions/), and append or
override the resource value in telemetry data with this information.

## Supported detectors

### Environment Variable

Reads resource information from the `OTEL_RESOURCE_ATTRIBUTES` environment
variable. This is expected to be in the format `<key1>=<value1>,<key2>=<value2>,...`, the
details of which are currently pending confirmation in the OpenTelemetry specification.

Example:

```yaml
processors:
  resourcedetection/env:
    detectors: [env]
    timeout: 2s
    override: false
```

### System metadata

Note: use the Docker detector (see below) if running the Collector as a Docker container.

Queries the host machine to retrieve the following resource attributes:

    * host.name
    * os.type

By default `host.name` is being set to FQDN if possible, and a hostname provided by OS used as fallback.
This logic can be changed with `hostname_sources` configuration which is set to `["dns", "os"]` by default.

Use the following config to avoid getting FQDN and apply hostname provided by OS only:

```yaml
processors:
  resourcedetection/system:
    detectors: ["system"]
    system:
      hostname_sources: ["os"]
```

* all valid options for `hostname_sources`:
    * "dns"
    * "os"
    * "cname"
    * "lookup"

#### Hostname Sources

##### dns

The "dns" hostname source uses multiple sources to get the fully qualified domain name. First, it looks up the
host name in the local machine's `hosts` file. If that fails, it looks up the CNAME. Lastly, if that fails,
it does a reverse DNS query. Note: this hostname source may produce unreliable results on Windows. To produce
a FQDN, Windows hosts might have better results using the "lookup" hostname source, which is mentioned below.

##### os

The "os" hostname source provides the hostname provided by the local machine's kernel.

##### cname

The "cname" hostname source provides the canonical name, as provided by net.LookupCNAME in the Go standard library.
Note: this hostname source may produce unreliable results on Windows.

##### lookup

The "lookup" hostname source does a reverse DNS lookup of the current host's IP address.

### Docker metadata

Queries the Docker daemon to retrieve the following resource attributes from the host machine:

    * host.name
    * os.type

You need to mount the Docker socket (`/var/run/docker.sock` on Linux) to contact the Docker daemon.
Docker detection does not work on macOS.

Example:

```yaml
processors:
  resourcedetection/docker:
    detectors: [env, docker]
    timeout: 2s
    override: false
```

### GCE Metadata

Uses the [Google Cloud Client Libraries for Go](https://github.com/googleapis/google-cloud-go)
to read resource information from the [GCE metadata server](https://cloud.google.com/compute/docs/storing-retrieving-metadata) to retrieve the following resource attributes:

    * cloud.provider ("gcp")
    * cloud.platform ("gcp_compute_engine")
    * cloud.account.id
    * cloud.region
    * cloud.availability_zone
    * host.id
    * host.image.id
    * host.type

Example:

```yaml
processors:
  resourcedetection/gce:
    detectors: [env, gce]
    timeout: 2s
    override: false
```

### GKE: Google Kubernetes Engine

    * cloud.provider ("gcp")
    * cloud.platform ("gcp_gke")
    * k8s.cluster.name (name of the GKE cluster)

Example:

```yaml
processors:
  resourcedetection/gke:
    detectors: [env, gke]
    timeout: 2s
    override: false
```


### AWS EC2

Uses [AWS SDK for Go](https://docs.aws.amazon.com/sdk-for-go/api/aws/ec2metadata/) to read resource information from the [EC2 instance metadata API](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-metadata.html) to retrieve the following resource attributes:

    * cloud.provider ("aws")
    * cloud.platform ("aws_ec2")
    * cloud.account.id
    * cloud.region
    * cloud.availability_zone
    * host.id
    * host.image.id
    * host.name
    * host.type

It also can optionally gather tags for the EC2 instance that the collector is running on.
Note that in order to fetch EC2 tags, the IAM role assigned to the EC2 instance must have a policy that includes the `ec2:DescribeTags` permission.

EC2 custom configuration example:
```yaml
processors:
  resourcedetection/ec2:
    detectors: ["ec2"]
      ec2:
        # A list of regex's to match tag keys to add as resource attributes can be specified
        tags:
          - ^tag1$
          - ^tag2$
          - ^label.*$
```

If you are using a proxy server on your EC2 instance, it's important that you exempt requests for instance metadata as [described in the AWS cli user guide](https://github.com/awsdocs/aws-cli-user-guide/blob/a2393582590b64bd2a1d9978af15b350e1f9eb8e/doc_source/cli-configure-proxy.md#using-a-proxy-on-amazon-ec2-instances). Failing to do so can result in proxied or missing instance data.

### Amazon ECS

Queries the [Task Metadata Endpoint](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-metadata-endpoint.html) (TMDE) to record information about the current ECS Task. Only TMDE V4 and V3 are supported.

    * cloud.provider ("aws")
    * cloud.platform ("aws_ecs")
    * cloud.account.id
    * cloud.region
    * cloud.availability_zone
    * aws.ecs.cluster.arn
    * aws.ecs.task.arn
    * aws.ecs.task.family
    * aws.ecs.task.revision
    * aws.ecs.launchtype (V4 only)
    * aws.log.group.names (V4 only)
    * aws.log.group.arns (V4 only)
    * aws.log.stream.names (V4 only)
    * aws.log.stream.arns (V4 only)

Example:

```yaml
processors:
  resourcedetection/ecs:
    detectors: [env, ecs]
    timeout: 2s
    override: false
```

### Amazon Elastic Beanstalk

Reads the AWS X-Ray configuration file available on all Beanstalk instances with [X-Ray Enabled](https://docs.aws.amazon.com/elasticbeanstalk/latest/dg/environment-configuration-debugging.html).

    * cloud.provider ("aws")
    * cloud.platform ("aws_elastic_beanstalk")
    * deployment.environment
    * service.instance.id
    * service.version

Example:

```yaml
processors:
  resourcedetection/elastic_beanstalk:
    detectors: [env, elastic_beanstalk]
    timeout: 2s
    override: false
```

### Amazon EKS

    * cloud.provider ("aws")
    * cloud.platform ("aws_eks")

Example:

```yaml
processors:
  resourcedetection/eks:
    detectors: [env, eks]
    timeout: 2s
    override: false
```

### Azure

Queries the [Azure Instance Metadata Service](https://aka.ms/azureimds) to retrieve the following resource attributes:

    * cloud.provider ("azure")
    * cloud.platform ("azure_vm")
    * cloud.region
    * cloud.account.id (subscription ID)
    * host.id (virtual machine ID)
    * host.name
    * azure.vm.size (virtual machine size)
    * azure.vm.scaleset.name (name of the scale set if any)
    * azure.resourcegroup.name (resource group name)

Example:

```yaml
processors:
  resourcedetection/azure:
    detectors: [env, azure]
    timeout: 2s
    override: false
```

### Azure AKS

  * cloud.provider ("azure")
  * cloud.platform ("azure_aks")

```yaml
processors:
  resourcedetection/aks:
    detectors: [env, aks]
    timeout: 2s
    override: false
```

### Consul

Queries a [consul agent](https://www.consul.io/docs/agent) and reads its' [configuration endpoint](https://www.consul.io/api-docs/agent#read-configuration) to retrieve the following resource attributes:

  * cloud.region (consul datacenter)
  * host.id (consul node id)
  * host.name (consul node name)
  * *exploded consul metadata* - reads all key:value pairs in [consul metadata](https://www.consul.io/docs/agent/options#_node_meta) into label:labelvalue pairs.

```yaml
processors:
  resourcedetection/consul:
    detectors: [env, consul]
    timeout: 2s
    override: false
```

## Configuration

```yaml
# a list of resource detectors to run, valid options are: "env", "system", "gce", "gke", "ec2", "ecs", "elastic_beanstalk", "eks", "azure"
detectors: [ <string> ]
# determines if existing resource attributes should be overridden or preserved, defaults to true
override: <bool>
# When included, only attributes in the list will be appened.  Applies to all detectors.
attributes: [ <string> ]
```

## Ordering

Note that if multiple detectors are inserting the same attribute name, the first detector to insert wins. For example if you had `detectors: [eks, ec2]` then `cloud.platform` will be `aws_eks` instead of `ec2`. The below ordering is recommended.

### GCP

* gke
* gce

### AWS

* elastic_beanstalk
* eks
* ecs
* ec2

The full list of settings exposed for this extension are documented [here](./config.go)
with detailed sample configurations [here](./testdata/config.yaml).

[beta]:https://github.com/open-telemetry/opentelemetry-collector#beta
[contrib]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
