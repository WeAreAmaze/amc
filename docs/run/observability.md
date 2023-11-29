# Observability with Prometheus & Grafana

amc provides a variety of metrics, listed here. They can be served from an HTTP endpoint by adding the --metrics flag:

```bash
amazechain --metrics --metrics.addr '0.0.0.0'  --metrics.port '6060'
```

Now, while the node is running, you can use the curl command to access the endpoint you provided to the --metrics.port  flag to get a text dump of the metrics at that point:

```bash
curl 127.0.0.1:6060/debug/metrics/prometheus
```

The response is quite descriptive, but it might be verbose. Moreover, it's just a snapshot of the metrics at the time you curled the endpoint.

You can run the following command in a separate terminal to periodically poll the endpoint, and print the values (without the header text) to the terminal:

```bash
while true; do date; curl -s 127.0.0.1:6060/debug/metrics/prometheus | grep -Ev '^(#|$)' | sort; echo; sleep 10; done
```

We're finally making progress! However, as a final step, wouldn't it be great to see how these metrics evolve over time (generally in a GUI)?

## Prometheus & Grafana

We're going to use Prometheus to collect metrics from the endpoint we set up, and use Grafana to scrape the metrics from Prometheus and define a dashboard.

Start by installing both Prometheus and Grafana, for instance via Homebrew:

```bash
brew update
brew install prometheus
brew install grafana
```

Then, kick off the Prometheus and Grafana services:

```bash
brew services start prometheus
brew services start grafana
```

This will start a Prometheus service which [by default scrapes itself about the current instance](https://prometheus.io/docs/introduction/first_steps/#:~:text=The%20job%20contains%20a%20single,%3A%2F%2Flocalhost%3A9090%2Fmetrics.). So you'll need to change its config to hit your Reth nodes metrics endpoint at `localhost:9001` which you set using the `--metrics` flag.

You can find an example config for the Prometheus service in the repo here: [`etc/prometheus/prometheus.yml`](https://github.com/paradigmxyz/reth/blob/main/etc/prometheus/prometheus.yml)

Depending on your installation you may find the config for your Prometheus service at:

- OSX: `/opt/homebrew/etc/prometheus.yml`
- Linuxbrew: `/home/linuxbrew/.linuxbrew/etc/prometheus.yml`
- Others: `/usr/local/etc/prometheus/prometheus.yml`

Next, open up "localhost:3000" in your browser, which is the default URL for Grafana. Here, "admin" is the default for both the username and password.

Once you've logged in, click on the gear icon in the lower left, and select "Data Sources". Click on "Add data source", and select "Prometheus" as the type. In the HTTP URL field, enter `http://localhost:9090`. Finally, click "Save & Test".

As this might be a point of confusion, `localhost:9001`, which we supplied to `--metrics`, is the endpoint that Reth exposes, from which Prometheus collects metrics. Prometheus then exposes `localhost:9090` (by default) for other services (such as Grafana) to consume Prometheus metrics.

To configure the dashboard in Grafana, click on the squares icon in the upper left, and click on "New", then "Import". From there, click on "Upload JSON file", and select the example file in [`reth/etc/grafana/dashboards/overview.json`](https://github.com/paradigmxyz/reth/blob/main/etc/grafana/dashboards/overview.json). Finally, select the Prometheus data source you just created, and click "Import".

And voilá, you should see your dashboard! If you're not yet connected to any peers, the dashboard will look like it's in an empty state, but once you are, you should see it start populating with data.

## Conclusion

In this runbook, we took you through starting the node, exposing different log levels, exporting metrics, and finally viewing those metrics in a Grafana dashboard.

This will all be very useful to you, whether you're simply running a home node and want to keep an eye on its performance, or if you're a contributor and want to see the effect that your (or others') changes have on Reth's operations.

[installation]: ../installation/installation.md
