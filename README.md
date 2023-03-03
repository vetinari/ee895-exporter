# EE895-exporter

A simple [prometheus exporter](https://prometheus.io/docs/instrumenting/exporters/) for 
reading sensor data from a [EE895-M16HV2 module](https://buyzero.de/products/ee895-m16hv2)
such as this [Raspberry PI Board](https://buyzero.de/products/raspberry-pi-co2-sensor-breakout-board).

It can additionally send the data to an MQTT broker, see [MQTT](#mqtt) section
on how to configure the broker and topics.

Based on the [ee895 python example](https://github.com/pi3g/ee895-python-example).

## Config

Run the exporter with `--config.file=/path/to/config.yml`, one example is [available](./ee895-exporter.yaml)

## MQTT

The MQTT broker is configured in the *mqtt* section. An example config looks like:
```yaml
mqtt:
  enabled: true
  broker:
    host: 192.168.5.33
    port: 1883
    username: brickd
    password: brickd_pass
    client_id: brickd_exporter
  topic: i2c/ee895
```

**Note**: if you're running multiple exporter each one must get a unique `client_id`.

The `mqtt.topic` sets the (full) topic where the metrics are reported to. 

## Building for Raspberry

Checkout the repo and run

```bash
GOARCH=arm GOARM=6 go build
```

and copy over to your raspberry :)
