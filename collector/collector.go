// Copyright 2015 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package collector

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/cadvisor/info/v2"
)

type GenericCollector struct {
	//name of the collector
	name string

	//holds information extracted from the config file for a collector
	configFile Config
}

type Config struct {
	//the endpoint to hit to scrape metrics
	Endpoint string `json:"endpoint"`

	//holds information about different metrics that can be collected
	MetricsConfig []MetricConfig `json:"metricsConfig"`
}

// metricConfig holds information extracted from the config file about a metric
type MetricConfig struct {
	//the name of the metric
	Name string `json:"name"`

	//enum type for the metric type
	MetricType MetricType `json:"metricType"`

	//data type of the metric (eg: integer, string)
	Units string `json:"units"`

	//the frequency at which the metric should be collected (in seconds)
	PollingFrequency time.Duration `json:"pollingFrequency"`

	//the regular expression that can be used to extract the metric
	Regex string `json:"regex"`
}

// MetricType is an enum type that lists the possible type of the metric
type MetricType string

const (
	Counter MetricType = "counter"
	Gauge   MetricType = "gauge"
)

//Returns a new collector using the information extracted from the configfile
func NewCollector(collectorName string, configfile string) (*GenericCollector, error) {
	configFile, err := ioutil.ReadFile(configfile)
	if err != nil {
		return nil, err
	}

	var configInJSON Config
	err = json.Unmarshal(configFile, &configInJSON)
	if err != nil {
		return nil, err
	}

	return &GenericCollector{
		name: collectorName, configFile: configInJSON,
	}, nil
}

//Returns name of the collector
func (collector *GenericCollector) Name() string {
	return collector.name
}

//Returns the next collection time and collected metrics for the collector; Returns the next collection time and an error message in case of any error during metrics collection
func (collector *GenericCollector) Collect() (time.Time, []v2.Metric, error) {
	minNextColTime := collector.configFile.MetricsConfig[0].PollingFrequency

	for _, metricConfig := range collector.configFile.MetricsConfig {
		if metricConfig.PollingFrequency < minNextColTime {
			minNextColTime = metricConfig.PollingFrequency
		}
	}
	currentTime := time.Now()
	nextCollectionTime := currentTime.Add(time.Duration(minNextColTime * time.Second))

	uri := collector.configFile.Endpoint
	response, err := http.Get(uri)
	if err != nil {
		return nextCollectionTime, nil, err
	}

	pageContent, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nextCollectionTime, nil, err
	}

	lines := strings.Split(string(pageContent), "\n")

	var metricType v2.MetricType
	metrics := make([]v2.Metric, len(collector.configFile.MetricsConfig))

	for ind, metricConfig := range collector.configFile.MetricsConfig {
		metricValue := make([]v2.IntPoint, 1)
		regex, err := regexp.Compile(metricConfig.Regex)
		if err != nil {
			return nextCollectionTime, nil, err
		}

		var matchString []string
		for _, line := range lines {
			matchString = regex.FindStringSubmatch(line)
			if matchString != nil {
				regVal, _ := strconv.ParseInt(strings.Trim(matchString[1], " "), 10, 64)
				metricValue[0].Value = regVal
				metricValue[0].Timestamp = currentTime
				break
			}
		}

		if matchString == nil {
			return nextCollectionTime, nil, errors.New("No match found for metric regexp: " + metricConfig.Regex)
		}

		metrics[ind].Name = metricConfig.Name
		if metricConfig.MetricType == "gauge" {
			metricType = v2.MetricGauge
		} else if metricConfig.MetricType == "counter" {
			metricType = v2.MetricCumulative
		}
		metrics[ind].Type = metricType
		metrics[ind].IntPoints = metricValue
		metrics[ind].Labels = nil
		metrics[ind].FloatPoints = nil
	}

	return nextCollectionTime, metrics, nil
}
