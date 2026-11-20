# Monitoring

This directory contains sample configurations for third party monitoring: Grafana dashboards in `grafana-dashboards`,
Prometheus scrape and alert rules in `prometheus.yml` and `rules`, and Splunk dashboards in `splunk-dashboard`.

### Making dashboard changes

Dashboard modifications happen in the Grafana web interface. To save changes, click on the "Save dashboard" icon in the
top right menu. Then proceed to export the dashboard as a JSON file by clicking the "Save JSON to file" button. Lastly,
move this file to the `grafana-dashboards` folder.
