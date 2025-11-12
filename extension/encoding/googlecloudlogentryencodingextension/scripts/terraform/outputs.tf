output "region" {
  description = "Region where the managed instance group is deployed."
  value       = var.region
}

output "mig_name" {
  description = "Name of the regional managed instance group hosting fixture instances."
  value       = google_compute_region_instance_group_manager.fixture_mig.name
}

output "subnet_name" {
  description = "Subnet name used for flow log generation and filtering."
  value       = google_compute_subnetwork.fixture_subnet.name
}

