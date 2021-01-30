// Copyright 2016 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build !nocpu

package collector

import (
	"bufio"
	"os/exec"
	"strconv"
	"strings"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

type powermetricsCollector struct {
	logger  log.Logger
	cputemp float64
}

func init() {
	registerCollector("powermetrics", defaultEnabled, NewPowermetricsCollector)
}

var (
	temperature = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "powermetrics", "temperature"),
		"Seconds the CPUs spent in each mode.",
		[]string{"point"}, nil,
	)
)

func NewPowermetricsCollector(logger log.Logger) (Collector, error) {
	r := &powermetricsCollector{
		logger:  logger,
		cputemp: 0,
	}
	level.Info(logger).Log("starting")
	go func() {
		cmd := exec.Command("sudo", "powermetrics")
		stdoutIn, _ := cmd.StdoutPipe()
		err := cmd.Start()
		if err != nil {
			level.Error(logger).Log("failed to start sudo powermetrics", err)
		}
		stdoutScanner := bufio.NewScanner(stdoutIn)
		for stdoutScanner.Scan() {
			line := stdoutScanner.Text()
			colspl := strings.Split(line, ": ")
			level.Info(logger).Log("msg", strings.Join(colspl, "|"))
			if len(colspl) == 2 && colspl[0] == "CPU die temperature" {
				spl := strings.Split(colspl[1], " ")
				level.Info(logger).Log("msg", strings.Join(spl, "|"))
				x, err2 := strconv.ParseFloat(spl[0], 64)
				if err2 == nil {
					r.cputemp = x
					level.Info(logger).Log("cpu temp", r.cputemp)
				}
			}

		}
	}()
	return r, nil
}

func (p *powermetricsCollector) Update(ch chan<- prometheus.Metric) error {
	ch <- prometheus.MustNewConstMetric(temperature, prometheus.GaugeValue, p.cputemp, "cpu")
	return nil
}
