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
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

type parameter struct {
	sourceName string
	target     *prometheus.Desc
	field      string
	value      float64
	populated  bool
	mu         *sync.Mutex
}

type powermetricsCollector struct {
	logger log.Logger
}

var (
	temperature = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "powermetrics", "temperature"),
		"Temperature reading",
		[]string{"point"}, nil,
	)
	parameters = [...]*parameter{
		&parameter{
			sourceName: "CPU die temperature",
			target:     temperature,
			field:      "cpu",
		},
		&parameter{
			sourceName: "GPU die temperature",
			target:     temperature,
			field:      "gpu",
		},
	}
)

func init() {
	registerCollector("powermetrics", defaultEnabled, NewPowermetricsCollector)
}

func setParameter(p *parameter, x float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.value = x
	p.populated = true
}

func readParameter(p *parameter) (float64, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.value, p.populated

}

func NewPowermetricsCollector(logger log.Logger) (Collector, error) {
	r := &powermetricsCollector{
		logger: logger,
	}
	for _, parameter := range parameters {
		parameter.mu = &sync.Mutex{}
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
			level.Info(logger).Log("msg", "fields", strings.Join(colspl, "|"))
			for _, parameter := range parameters {
				if len(colspl) == 2 && colspl[0] == parameter.sourceName {
					spl := strings.Split(colspl[1], " ")
					x, err2 := strconv.ParseFloat(spl[0], 64)

					if err2 == nil {
						setParameter(parameter, x)
					}
					level.Info(logger).Log("msg", fmt.Sprintf("parameter %vc field %s value [%f] populate [%t]", parameter.target, parameter.field, parameter.value, parameter.populated))
				}
			}
		}
	}()
	return r, nil
}

func (p *powermetricsCollector) Update(ch chan<- prometheus.Metric) error {
	for _, parameter := range &parameters {
		x, populated := readParameter(parameter)
		if populated {
			ch <- prometheus.MustNewConstMetric(parameter.target, prometheus.GaugeValue, x, parameter.field)
		}
	}

	return nil
}
