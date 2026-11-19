package main

import (
	"runtime"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	tempGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "hp3001_temperature_celsius",
		Help: "Temperature measured by the HP3001, in degrees Celsius.",
	}, []string{"device", "channel"})

	humGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "hp3001_humidity_percent",
		Help: "Relative humidity, as measured by the HP3001, in percent.",
	}, []string{"device", "channel"})

	buildInfoGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "hp3001_exporter_build_info",
		Help: "Build information about the HP3001 exporter. Always 1.",
	}, []string{"version", "commit", "go_version"})
)

func init() {
	prometheus.MustRegister(tempGauge, humGauge, buildInfoGauge)
}

// registerBuildInfo sets the build_info metric to 1 with labels
// describing the current build. Call once at startup.
func registerBuildInfo() {
	buildInfoGauge.WithLabelValues(
		shortVersion(), Commit, runtime.Version(),
	).Set(1)
}
