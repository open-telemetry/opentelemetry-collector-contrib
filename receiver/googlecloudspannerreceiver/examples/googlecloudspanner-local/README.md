This is a sample docker service, to show how googlecloudspannerreceiver can work along with prometheus exporter and grafana.
docker-compose.yml ties together the above 3 components.
In order to connect to a different cloud spanner project, we can use either of service account key or ADC and modify the collector, prometheus and grafana configurations accordingly.

Note the following locations of the config files : 
1. Prometheus : prometheus/config.yml
2. Grafana : grafana/provisioning/dashboards/dashboards.yml & grafana/provisioning/datasources
3. Collector : collector/config.yml
4. Docker: ./docker-compose.yml

IMPORTANT : In all the above configuration files, replace all placeholders with required values.

Docker instructions : 
Run containers with logs per each container are displayed in this one terminal window.
`docker compose up`

Run in detached mode (no logs output). Look for logs in Docke Desktop UI.
`docker compose up -d`

Run single container.
`docker compose up grafana`

Stop containers with all data saved in volumes.
`docker compose down`

Stop containers with volumes removal. Usefull for clean starts.
`docker compose down -v`
