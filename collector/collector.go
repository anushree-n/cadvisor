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
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/cadvisor/info/v2"
)

func NewNginxCollector() (*Collector, error) {
	return &Collector{
		name: "nginx", configFile: Config{}, nextCollectionTime: time.Now(), err: nil,
	}, nil
}

//Returns name of the collector
func (collector *Collector) Name() string {
	return collector.name
}

func (collector *Collector) Collect() (time.Time, []v2.Metric, error) {
	currentTime := time.Now()
	collector.nextCollectionTime = currentTime.Add(time.Duration(10 * time.Second))

	uri := "http://" + collector.configFile.Endpoint
	uri = strings.Replace(uri, "host", "localhost", 1)
	uri = strings.Replace(uri, "port", "8000", 1)

	response, err := http.Get(uri)
	if err != nil {
		return collector.nextCollectionTime, nil, err
	}

	pageContent, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return collector.nextCollectionTime, nil, err
	}

	lines := strings.Split(string(pageContent), "\n")

	var metricType v2.MetricType
	var regex *regexp.Regexp

	numOfMetrics := len(collector.configFile.MetricsConfig)
	metrics := make([]v2.Metric, numOfMetrics)

	for ind, metricConfig := range collector.configFile.MetricsConfig {
		metrics[ind].Name = metricConfig.Name
		metrics[ind].Type = metricType
		metrics[ind].Labels = nil
		metrics[ind].FloatPoints = nil

		metricValue := make([]v2.IntPoint, 1)
		regex = regexp.MustCompile(metricConfig.Regex)

		for _, line := range lines {
			matchString := regex.FindStringSubmatch(line)
			if matchString != nil {
				regVal, _ := strconv.ParseInt(strings.Trim(matchString[1], " "), 10, 64)
				metricValue[0].Value = regVal
				metricValue[0].Timestamp = time.Now()

				metrics[ind].IntPoints = metricValue
				break
			}
		}
	}

	return collector.nextCollectionTime, metrics, nil
}
