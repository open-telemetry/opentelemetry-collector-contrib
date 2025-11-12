variable "project_id" {
  description = "Google Cloud project ID where fixture infrastructure will be created."
  type        = string

  validation {
    condition     = length(trimspace(var.project_id)) > 0
    error_message = "project_id must be a non-empty string."
  }
}

variable "region" {
  description = "Google Cloud region used for regional resources such as the managed instance group."
  type        = string

  validation {
    condition     = length(trimspace(var.region)) > 0
    error_message = "region must be a non-empty string."
  }
}

variable "zone" {
  description = "Default Google Cloud zone used for zonal API operations (e.g., instance metadata updates)."
  type        = string

  validation {
    condition     = length(trimspace(var.zone)) > 0
    error_message = "zone must be a non-empty string."
  }
}

variable "resource_prefix" {
  description = "Prefix applied to all fixture infrastructure resources."
  type        = string
  default     = "gcp-fixture"

  validation {
    condition     = length(trimspace(var.resource_prefix)) > 0
    error_message = "resource_prefix cannot be empty."
  }
}

locals {
  network_name           = "${var.resource_prefix}-network"
  subnet_name            = "${var.resource_prefix}-subnet"
  mig_name               = "${var.resource_prefix}-mig"
  template_name          = "${var.resource_prefix}-template"
  firewall_internal_name = "${var.resource_prefix}-allow-internal"
  firewall_ssh_name      = "${var.resource_prefix}-allow-ssh"
  base_instance_name     = "${var.resource_prefix}-vm"
  instance_tag           = var.resource_prefix
}

