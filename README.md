# EE895-exporter

A simple [prometheus exporter](https://prometheus.io/docs/instrumenting/exporters/) for 
reading sensor data from a [EE895-M16HV2 module](https://buyzero.de/products/ee895-m16hv2)
such as this [Raspberry PI Board](https://buyzero.de/products/raspberry-pi-co2-sensor-breakout-board).

Based on the [ee895 python example](https://github.com/pi3g/ee895-python-example).

## Config

Run the exporter with `--config.file=/path/to/config.yml`, one example is [available](./ee895-exporter.yaml)

## Building for Raspberry

Checkout the repo and run

```bash
GOARCH=arm GOARM=6 go build
```

and copy over to your raspberry :)


