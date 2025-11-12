# VPC Flow Fixture Terraform Configuration

This module provisions the Google Cloud infrastructure required to capture VPC
flow logs for the
`googlecloudlogentryencodingextension` test fixtures. It replaces the manual
`gcloud`-heavy setup with declarative Terraform resources while preserving the
existing Go helper and traffic generation scripts.

## Prerequisites

- Terraform **v1.5+**
- Google provider **v5.0+** (downloaded automatically by Terraform)
- An authenticated `gcloud` session (`gcloud auth login`) with access to the
  target project
- The Google Cloud APIs used by the resources must be enabled:
  `compute.googleapis.com`, `logging.googleapis.com`

> The helper scripts use the Go traffic runner to connect to instances via SSH
> and generate traffic after Terraform completes.

## Quick Start (recommended)

1. Copy the example environment file and adjust the values:

   ```bash
   cp scripts/vpc_flow_fixtures.env.example scripts/vpc_flow_fixtures.env
   $EDITOR scripts/vpc_flow_fixtures.env
   ```

2. Generate fixtures (creates infrastructure via Terraform and triggers traffic):

   ```bash
   scripts/generate_vpc_flow_fixtures.sh
   ```

3. Export logs (unchanged from previous workflow):

   ```bash
   scripts/export_vpc_flow_logs.sh
   ```

4. Tear everything down when finished (uses `terraform destroy` under the hood):

   ```bash
   scripts/teardown_vpc_flow_fixtures.sh --dry-run=false
   ```

## Manual Terraform Usage

The wrapper scripts above handle the common workflow, but you can run Terraform
directly for debugging:

```bash
cd extension/encoding/googlecloudlogentryencodingextension/scripts/terraform
cp terraform.tfvars.example terraform.tfvars
# edit terraform.tfvars with your project/region/zone/prefix

terraform init
terraform plan
terraform apply

# ... run helper scripts as needed ...

terraform destroy
```

The `terraform.tfvars` file (stored in `scripts/terraform/`) drives both manual
runs and the wrapper scripts. If you invoke the scripts, they will respect any
existing `terraform.tfvars`.

## Variables

| Name             | Type   | Default       | Description                                                  |
| ---------------- | ------ | ------------- | ------------------------------------------------------------ |
| `project_id`     | string | _(required)_  | Google Cloud project that hosts the fixture infrastructure.  |
| `region`         | string | _(required)_  | Region for the regional managed instance group.              |
| `zone`           | string | _(required)_  | Default zone used by the provider for zonal API operations.  |
| `resource_prefix`| string | `gcp-fixture` | Prefix applied to all Terraform-managed resources.           |

## Outputs

| Name             | Description                                                  |
| ---------------- | ------------------------------------------------------------ |
| `region`         | Region where the managed instance group resides.             |
| `mig_name`       | Managed instance group name consumed by the Go helper.       |
| `subnet_name`    | Subnet used for log filtering and traffic generation.        |

## Resource Overview

Terraform manages the following resources:

- VPC network (`google_compute_network`)
- Subnet with flow logs enabled (`google_compute_subnetwork`)
- Internal and SSH firewall rules (`google_compute_firewall`)
- Instance template (`google_compute_instance_template`)
- Regional managed instance group with two instances
  (`google_compute_region_instance_group_manager`)