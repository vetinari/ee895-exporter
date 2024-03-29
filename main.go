package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	// _ "net/http/pprof"

	"github.com/d2r2/go-i2c"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

var i2cBus = 1
var i2cAddr i2cAddress = 0x5E
var hostname string

type i2cAddress uint8

func (i i2cAddress) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("\"0x%02X\"", i)), nil
}

func main() {
	config, err := parseConfig()
	if err != nil {
		log.Fatalf("Error loading configuration: %s", err)
	}
	if config.Collector.LogLevel == "" {
		config.Collector.LogLevel = "info"
	}
	lvl, err := log.ParseLevel(config.Collector.LogLevel)
	if err != nil {
		log.Errorf("failed to parse `log_level` %s: %s", config.Collector.LogLevel, err)
		lvl = log.InfoLevel
	}
	log.SetLevel(lvl)
	if config.Collector.LogFormat == "" {
		config.Collector.LogFormat = "text"
	}
	switch config.Collector.LogFormat {
	case "text":
		log.SetFormatter(new(log.TextFormatter))
	case "json":
		log.SetFormatter(new(log.JSONFormatter))
	default:
		log.Errorf("unknown formatter `%s`, using `text` as fallback", config.Collector.LogFormat)
		log.SetFormatter(new(log.TextFormatter))
	}

	hostname, err = os.Hostname()
	if err != nil {
		log.Errorf("failed to get hostname, setting to unknown: %s", err)
		hostname = "unknown"
	}

	var mqClient *Client
	if config.MQTT.Enabled {
		mqClient, err = NewClient(config.MQTT.Broker)
		if err != nil {
			log.Errorf("failed to get MQTT client: %s", err)
		}
		config.MQTT.Client = mqClient
	}

	ch := make(chan EE895Data)
	c := &Collector{Channel: ch, Labels: config.Collector.Labels, MQTT: config.MQTT, Topic: Topic(config.MQTT.Topic)}
	go c.Run()

	prometheus.MustRegister(c)

	listenAddress := config.Listen.Address

	http.Handle(config.Listen.MetricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, config.Listen.MetricsPath, http.StatusFound)
	})

	log.Printf("Starting i2c exporter on %q", listenAddress)

	if err := http.ListenAndServe(listenAddress, nil); err != nil {
		log.Fatalf("Cannot start I2C exporter: %s", err)
	}
}

type EE895Data struct {
	CO2         uint16  `json:"co2"`
	Temperature float64 `json:"temperature"`
	Pressure    float64 `json:"pressure"`
}

type MQTTData struct {
	CO2         uint16            `json:"co2"`
	Temperature float64           `json:"temperature"`
	Pressure    float64           `json:"pressure"`
	Hostname    string            `json:"hostname"`
	I2CBus      int               `json:"i2c_bus"`
	I2CAddr     i2cAddress        `json:"i2c_address"`
	Labels      map[string]string `json:"labels"`
}

type Collector struct {
	sync.Mutex
	Channel chan EE895Data
	Bus     *i2c.I2C
	Data    EE895Data
	Labels  map[string]string
	MQTT    MQTT
	Topic   Topic
}

func (c *Collector) Run() {
	bus, err := i2c.NewI2C(uint8(i2cAddr), i2cBus)
	if err != nil {
		log.Fatalf("failed to open I2C Bus: %s", err)
	}
	c.Bus = bus
	go func() {
		for data := range c.Channel {
			c.Lock()
			c.Data = data
			c.Unlock()
		}
	}()

	for {
		data, n, err := c.Bus.ReadRegBytes(0x0, 8)
		if err != nil {
			log.Warnf("failed to read data: %s", err)
			time.Sleep(15 * time.Second)
			continue
		}
		if n != 8 {
			log.Warnf("short read of %d bytes", n)
			time.Sleep(15 * time.Second)
			continue
		}

		co2 := binary.BigEndian.Uint16([]byte{data[0], data[1]})
		temp := float64(binary.BigEndian.Uint16([]byte{data[2], data[3]})) / 100.0
		pressure := float64(binary.BigEndian.Uint16([]byte{data[6], data[7]})) / 10.0

		log.Debugf("CO2: %d ppm | Temperature: %.2f °C | Pressure: %.1f hPa\n", co2, temp, pressure)

		c.Channel <- EE895Data{
			CO2:         co2,
			Temperature: temp,
			Pressure:    pressure,
		}

		if c.MQTT.Enabled {
			mqd := &MQTTData{
				CO2:         co2,
				Temperature: temp,
				Pressure:    pressure,
				Hostname:    hostname,
				I2CBus:      i2cBus,
				I2CAddr:     i2cAddr,
				Labels:      c.Labels,
			}
			go c.Publish(mqd)
		}
		time.Sleep(15 * time.Second)
	}
}

// Publish publishes to the MQTT broker.
func (c *Collector) Publish(data *MQTTData) {
	enc, err := json.Marshal(data)
	if err != nil {
		log.Errorf("failed to marshal json: %s", err)
		return
	}
	c.MQTT.Client.Publish(string(c.Topic), enc)
}

// Describe is part of the prometheus.Collector interface
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- prometheus.NewDesc("dummy", "dummy", nil, nil)
}

// Collect is part of the prometheus.Collector interface
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	c.Lock()
	defer c.Unlock()

	desc := prometheus.NewDesc(
		"i2c_co2_value",
		"CO2 Level in ppm",
		nil,
		c.Labels,
	)
	ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(c.Data.CO2))

	desc = prometheus.NewDesc(
		"i2c_temperature_value",
		"Temperature in °C",
		nil,
		c.Labels,
	)
	ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, c.Data.Temperature)

	desc = prometheus.NewDesc(
		"i2c_pressure_value",
		"Air Pressure in hPa",
		nil,
		c.Labels,
	)
	ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, c.Data.Pressure)
}
