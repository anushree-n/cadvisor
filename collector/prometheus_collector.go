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
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"
	//	"github.com/golang/glog"
	"github.com/google/cadvisor/info/v1"
)

type PrometheusCollector struct {
	//name of the collector
	name string

	//holds information extracted from the config file for a collector
	configFile Prometheus
}

//Returns a new collector using the information extracted from the configfile
func NewPrometheusCollector(collectorName string, configFile []byte) (*PrometheusCollector, error) {
	var configInJSON Prometheus
	err := json.Unmarshal(configFile, &configInJSON)
	if err != nil {
		return nil, err
	}

	//TODO : Add checks for validity of config file (eg : Accurate JSON fields)
	return &PrometheusCollector{
		name:       collectorName,
		configFile: configInJSON,
	}, nil
}

//Returns name of the collector
func (collector *PrometheusCollector) Name() string {
	return collector.name
}

func getMetricData(line string) string {
	fields := strings.Fields(line)
	data := fields[3]
	if len(fields) > 3 {
		for i, _ := range fields {
			if i > 3 {
				data = data + "_" + fields[i]
			}
		}
	}
	return strings.TrimSpace(data)
}

func (collector *PrometheusCollector) GetSpec() []v1.MetricSpec {
	specs := []v1.MetricSpec{}
	response, err := http.Get(collector.configFile.Endpoint)
	if err != nil {
		return specs
	}

	pageContent, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return specs
	}

	lines := strings.Split(string(pageContent), "\n")
	//if allMetrics {
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# HELP") {
			metUnit := getMetricData(lines[i])
			metType := getMetricData(lines[i+1])
			stopIndex := strings.Index(lines[i+2], "{")
			if stopIndex == -1 {
				stopIndex = strings.Index(lines[i+2], " ")
			}
			metName := strings.TrimSpace(lines[i+2][0:stopIndex])
			//	fmt.Println("Name :  ", metName)
			//	fmt.Println("Type :  ", metType)
			//	fmt.Println("Units:  ", metUnit)
			//	fmt.Println()
			spec := v1.MetricSpec{
				Name:   metName,
				Type:   v1.MetricType(metType),
				Format: "float",
				Units:  metUnit,
			}
			specs = append(specs, spec)
		}
	}
	//	}
	response.Body.Close()
	return specs
}

//Returns collected metrics and the next collection time of the collector
func (collector *PrometheusCollector) Collect(metrics map[string][]v1.MetricVal) (time.Time, map[string][]v1.MetricVal, error) {
	currentTime := time.Now()
	nextCollectionTime := currentTime.Add(time.Duration(collector.configFile.PollingFrequency))

	uri := collector.configFile.Endpoint
	response, err := http.Get(uri)
	if err != nil {
		return nextCollectionTime, nil, err
	}

	//defer response.Body.Close()

	pageContent, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nextCollectionTime, nil, err
	}

	lines := strings.Split(string(pageContent), "\n")

	for _, line := range lines {
		if line == "" {
			break
		}
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "# HELP") && !strings.HasPrefix(line, "# TYPE") {
			startLabelIndex := strings.Index(line, "{")
			spaceIndex := strings.Index(line, " ")
			if startLabelIndex == -1 {
				startLabelIndex = spaceIndex
			}
			metName := strings.TrimSpace(line[0:startLabelIndex])
			var metLabel string
			if startLabelIndex+1 <= spaceIndex-1 {
				metLabel = strings.TrimSpace(line[startLabelIndex+1 : spaceIndex-1])
			}
			metVal, err := strconv.ParseFloat(line[spaceIndex+1:], 64)
			if err != nil {
				return nextCollectionTime, nil, err
			}
			//	fmt.Println("Name :  ", metName)
			//	fmt.Println("Label:  ", metLabel)
			//	fmt.Println("Value:  ", metVal)
			//	fmt.Println()
			metric := v1.MetricVal{
				Label:      metLabel,
				FloatValue: metVal,
				Timestamp:  currentTime,
			}
			metrics[metName] = append(metrics[metName], metric)
		}
	}
	response.Body.Close()
	return nextCollectionTime, metrics, nil
}
