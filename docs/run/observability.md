# Observability with Prometheus & Grafana

AMC provides a variety of metrics, listed here. To serve them from an HTTP endpoint, add the `--metrics` flag:

```bash
amazechain --metrics --metrics.addr '0.0.0.0' --metrics.port '6060'
```

While the node is running, you can use the `curl` command to access the endpoint specified by the `--metrics.port` flag to obtain a text dump of the metrics:

```bash
curl 127.0.0.1:6060/debug/metrics/prometheus
```

The response is quite descriptive but may be verbose. It represents just a snapshot of the metrics at the time you accessed the endpoint.

To periodically poll the endpoint and print the values (excluding the header text) to the terminal, run the following command in a separate terminal:

```bash
while true; do date; curl -s 127.0.0.1:6060/debug/metrics/prometheus | grep -Ev '^(#|$)' | sort; echo; sleep 10; done
```

We're making progress! For a visual representation of how these metrics evolve over time (typically in a GUI), follow the next steps.

## Prometheus & Grafana

We will use Prometheus to collect metrics from the endpoint we set up, and Grafana to scrape the metrics from Prometheus and display them on a dashboard.

First, install both Prometheus and Grafana, for instance via Homebrew:

```bash
brew update
brew install prometheus
brew install grafana
```

Then, start the Prometheus and Grafana services:

```bash
brew services start prometheus
brew services start grafana
```

By default, Prometheus scrapes metrics about its instance. You'll need to modify its configuration to scrape metrics from your node's endpoint at `localhost:9001` set by the `--metrics` flag.

You can find an example configuration for the Prometheus service in the repo here: [`etc/prometheus/prometheus.yml`](https://github.com/paradigmxyz/reth/blob/main/etc/prometheus/prometheus.yml)

Depending on your installation, the config file for Prometheus might be located at:
- OSX: `/opt/homebrew/etc/prometheus.yml`
- Linuxbrew: `/home/linuxbrew/.linuxbrew/etc/prometheus.yml`
- Others: `/usr/local/etc/prometheus/prometheus.yml`

Next, open `localhost:3000` in your browser, the default URL for Grafana. The default username and password are both "admin".

After logging in, click on the gear icon in the lower left, and select "Data Sources". Then click on "Add data source", choose "Prometheus" as the type, and in the HTTP URL field, enter `http://localhost:9090`. Click "Save & Test".

Note that `localhost:9001` is the endpoint that Reth exposes for Prometheus to collect metrics from, while Prometheus serves these metrics at `localhost:9090` for Grafana to access.

To set up the dashboard in Grafana, click on the squares icon in the upper left, then "New", and "Import". Next, click on "Upload JSON file", and choose the example file from [`reth/etc/grafana/dashboards/overview.json`](https://github.com/paradigmxyz/reth/blob/main/etc/grafana/dashboards/overview.json). Select the Prometheus data source you just set up and click "Import".

Voilà, you should now see your dashboard! If you're not yet connected to any peers, the dashboard may initially appear empty, but it will start populating with data once connections are established.

## Conclusion

In this guide, we've walked you through starting a node, exposing various log levels, exporting metrics, and finally visualizing those metrics on a Grafana dashboard.

This information is invaluable, whether you're running a home node and want to monitor its performance, or you're a contributor interested in the impact of your changes—or those of others—on Reth's operations.

[installation]: ../installation/installation.md

