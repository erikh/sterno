package main

import (
	"crypto/rand"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Port      uint          `yaml:"port"`
	Namespace string        `yaml:"namespace"`
	Metrics   []Metric      `yaml:"metrics"`
	Interval  time.Duration `yaml:"interval"`
}

type Metric struct {
	Name        string  `yaml:"name"`
	Subsystem   string  `yaml:"subsystem"`
	Type        string  `yaml:"type"`
	Random      bool    `yaml:"random"`
	StaticValue float64 `yaml:"static_value"`
	RandomMin   uint64  `yaml:"random_min"`
	RandomMax   uint64  `yaml:"random_max"`
}

func buildMetrics(c Config) (*prometheus.Registry, error) {
	reg := prometheus.NewRegistry()

	for _, metric := range c.Metrics {
		if metric.Subsystem == "" {
			metric.Subsystem = "sterno"
		}

		switch metric.Type {
		case "gauge":
			gauge := prometheus.NewGauge(prometheus.GaugeOpts{
				Namespace: c.Namespace,
				Subsystem: metric.Subsystem,
				Name:      metric.Name,
			})

			reg.MustRegister(gauge)

			go func(gauge prometheus.Gauge, c Config, metric Metric) {
				for {
					if metric.Random {
						buf := make([]byte, 8)
						if _, err := rand.Reader.Read(buf); err != nil {
							log.Printf("Error while reading random data: %v", err)
							continue
						}

						res, err := binary.Uvarint(buf)
						if err < 0 {
							log.Printf("Error generating random number")
							continue
						}

						gauge.Set(float64((res % (metric.RandomMax - metric.RandomMin)) + metric.RandomMin))
					} else {
						gauge.Set(metric.StaticValue)
					}

					time.Sleep(c.Interval)
				}
			}(gauge, c, metric)

		default:
			return nil, fmt.Errorf("Type %q is not supported", metric.Type)
		}
	}

	return reg, nil
}

func main() {
	var (
		config = flag.String("config", "sterno.conf", "Path to sterno configuration file; YAML or JSON")
	)

	flag.Parse()

	out, err := os.ReadFile(*config)
	if err != nil {
		log.Fatal(err)
	}

	var c Config

	if err := yaml.Unmarshal(out, &c); err != nil {
		log.Fatal(err)
	}

	if c.Namespace == "" {
		c.Namespace = "sterno"
	}

	if c.Interval == 0 {
		c.Interval = time.Second
	}

	reg, err := buildMetrics(c)
	if err != nil {
		log.Fatal(err)
	}

	http.Handle("/metrics", promhttp.HandlerFor(
		reg,
		promhttp.HandlerOpts{
			// Opt into OpenMetrics to support exemplars.
			EnableOpenMetrics: true,
			// Pass custom registry
			Registry: reg,
		},
	))

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", c.Port), nil))
}
