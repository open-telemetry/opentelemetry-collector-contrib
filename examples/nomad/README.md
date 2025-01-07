# OpenTelemetry Collector Demo

This demo provides you with all you need to run the OpenTelemetry Collector on a local installation of HashiCorp [Nomad](https://nomadproject.io) using [Traefik](https://traefik.io) as your load balancer. It configures both HTTP and gRPC endpoints on Traefik.

For a detailed, step-by-step tutorial on how to run the files included in this directory, check out [Just-in-Time Nomad: Running the OpenTelemetry Collector on HashiCorp Nomad with HashiQube](https://storiesfromtheherd.com/just-in-time-nomad-running-the-opentelemetry-collector-on-hashicorp-nomad-with-hashiqube-4eaf009b8382).

The tutorial includes instructions on how to set up a local Nomad environment using [HashiQube](https://github.com/avillela/hashiqube) (a fork of [servian/hashiqube](https://github.com/servian/hashiqube)). It spins up a Vagrant VM running [Nomad](https://nomadproject.io), [Consul](https://www.consul.io),  [Vault](https://www.vaultproject.io) and [Traefik](https://traefik.io). Detailed instructions can be found in the project [readme](https://github.com/avillela/hashiqube#readme).


## Additional Resources

Check out the [OpenTelemetry Collector Nomad Pack](https://github.com/hashicorp/nomad-pack-community-registry/tree/main/packs/opentelemetry_collector) in the [Nomad Pack Community Registry](https://github.com/hashicorp/nomad-pack-community-registry).