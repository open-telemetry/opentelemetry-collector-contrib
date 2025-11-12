resource "google_compute_network" "fixture_network" {
  name                    = local.network_name
  auto_create_subnetworks = false
}

resource "google_compute_subnetwork" "fixture_subnet" {
  name          = local.subnet_name
  region        = var.region
  network       = google_compute_network.fixture_network.id
  ip_cidr_range = "10.10.0.0/20"

  log_config {
    aggregation_interval = "INTERVAL_5_MIN"
    flow_sampling        = 1.0
    metadata             = "INCLUDE_ALL_METADATA"
  }
}

resource "google_compute_firewall" "fixture_internal" {
  name    = local.firewall_internal_name
  network = google_compute_network.fixture_network.name

  direction = "INGRESS"
  source_ranges = [
    "10.10.0.0/20",
  ]

  allow {
    protocol = "tcp"
    ports    = ["0-65535"]
  }

  allow {
    protocol = "udp"
    ports    = ["0-65535"]
  }

  allow {
    protocol = "icmp"
  }
}

resource "google_compute_firewall" "fixture_ssh" {
  name    = local.firewall_ssh_name
  network = google_compute_network.fixture_network.name

  direction = "INGRESS"
  source_ranges = [
    "0.0.0.0/0",
  ]
  target_tags = [
    local.instance_tag,
  ]

  allow {
    protocol = "tcp"
    ports    = ["22"]
  }
}

resource "google_compute_instance_template" "fixture_template" {
  name = local.template_name

  machine_type = "e2-standard-2"

  disk {
    auto_delete  = true
    boot         = true
    source_image = "projects/debian-cloud/global/images/family/debian-12"
  }

  tags = [
    local.instance_tag,
  ]

  network_interface {
    network    = google_compute_network.fixture_network.id
    subnetwork = google_compute_subnetwork.fixture_subnet.id
    access_config {
      // Ephemeral external IP for outbound internet access (apt, curl).
    }
  }

  service_account {
    scopes = [
      "https://www.googleapis.com/auth/cloud-platform",
    ]
  }

  lifecycle {
    create_before_destroy = true
  }
}

resource "google_compute_region_instance_group_manager" "fixture_mig" {
  name               = local.mig_name
  base_instance_name = local.base_instance_name
  region             = var.region
  target_size        = 2

  distribution_policy_zones = [
    var.zone,
  ]

  version {
    name              = "primary"
    instance_template = google_compute_instance_template.fixture_template.self_link
  }

  update_policy {
    type                           = "PROACTIVE"
    minimal_action                 = "RESTART"
    most_disruptive_allowed_action = "RESTART"
    max_unavailable_fixed          = 2
  }
}

