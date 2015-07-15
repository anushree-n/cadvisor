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
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/cadvisor/info/v1"
)

type GenericCollector struct {
	//name of the collector
	name string

	//holds information extracted from the config file for a collector
	configFile Config

	//holds information necessary to extract metrics
	info *collectorInfo
}

type collectorInfo struct {
	//regular expresssions for all metrics
	regexps []*regexp.Regexp
}

//Returns a new collector using the information extracted from the configfile
func NewCollector(collectorName string, configfile string) (*GenericCollector, error) {
	configFile, err := ioutil.ReadFile(configfile)
	if err != nil {
		return nil, err
	}

	switch collectorName {
	case "Prometheus":
		var configInJSON Prometheus
		err = json.Unmarshal(configFile, &configInJSON)
		if err != nil {
			return nil, err
		}

		//TODO : Add checks for validity of config file (eg : Accurate JSON fields)

		if len(configInJSON.MetricsConfig) == 0 {
			return nil, fmt.Errorf("No metrics provided in config")
		}

		return &GenericCollector{
			name:       collectorName,
			configFile: Config{configInJSON},
			info:       &collectorInfo{},
		}, nil

	case "REST":
		var configInJSON REST
		err = json.Unmarshal(configFile, &configInJSON)
		if err != nil {
			return nil, err
		}

		//TODO : Add checks for validity of config file (eg : Accurate JSON fields)

		if len(configInJSON.MetricsConfig) == 0 {
			return nil, fmt.Errorf("No metrics provided in config")
		}
		regexprs := make([]*regexp.Regexp, len(configInJSON.MetricsConfig))

		for ind, metricConfig := range configInJSON.MetricsConfig {
			regexprs[ind], err = regexp.Compile(metricConfig.Regex)
			if err != nil {
				return nil, fmt.Errorf("Invalid regexp %v for metric %v", metricConfig.Regex, metricConfig.Name)
			}
		}

		return &GenericCollector{
			name:       collectorName,
			configFile: Config{configInJSON},
			info: &collectorInfo{
				regexps: regexprs},
		}, nil

	default:
		return nil, fmt.Errorf("No support for %v", collectorName)

	}
}

//Returns name of the collector
func (collector *GenericCollector) Name() string {
	return collector.name
}

func readSource(uri string) (string, error) {
	response, err := http.Get(uri)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	pageContent, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	return string(pageContent), nil
}

//Returns collected metrics and the next collection time of the collector
func (collector *GenericCollector) Collect() (time.Time, []v1.Metric, error) {
	switch collector.name {
	case "REST":
		config := collector.configFile.makeREST()

		currentTime := time.Now()
		nextCollectionTime := currentTime.Add(time.Duration(config.PollingFrequency * time.Second))

		uri := config.Source
		pageContent, err := readSource(uri)
		if err != nil {
			return nextCollectionTime, nil, err
		}

		metrics := make([]v1.Metric, len(config.MetricsConfig))
		var errorSlice []error

		for ind, metricConfig := range config.MetricsConfig {
			matchString := collector.info.regexps[ind].FindStringSubmatch(pageContent)
			if matchString != nil {
				if metricConfig.Units == "float" {
					regVal, err := strconv.ParseFloat(strings.TrimSpace(matchString[1]), 64)
					if err != nil {
						errorSlice = append(errorSlice, err)
					}
					metrics[ind].FloatPoints = []v1.FloatPoint{
						{Value: regVal, Timestamp: currentTime},
					}
				} else if metricConfig.Units == "integer" || metricConfig.Units == "int" {
					regVal, err := strconv.ParseInt(strings.TrimSpace(matchString[1]), 10, 64)
					if err != nil {
						errorSlice = append(errorSlice, err)
					}
					metrics[ind].IntPoints = []v1.IntPoint{
						{Value: regVal, Timestamp: currentTime},
					}
				} else {
					errorSlice = append(errorSlice, fmt.Errorf("Unexpected value of 'units' for metric '%v' in config ", metricConfig.Name))
				}
			} else {
				errorSlice = append(errorSlice, fmt.Errorf("No match found for regexp: %v for metric '%v' in config", metricConfig.Regex, metricConfig.Name))
			}

			metrics[ind].Name = metricConfig.Name
			if metricConfig.MetricType == "gauge" {
				metrics[ind].Type = v1.MetricGauge
			} else if metricConfig.MetricType == "counter" {
				metrics[ind].Type = v1.MetricCumulative
			}
		}

		return nextCollectionTime, metrics, compileErrors(errorSlice)

	case "Prometheus":
		config := collector.configFile.makePrometheus()

		currentTime := time.Now()
		nextCollectionTime := currentTime.Add(time.Duration(config.PollingFrequency * time.Second))

		uri := config.Source
		pageContent, err := readSource(uri)
		if err != nil {
			return nextCollectionTime, nil, err
		}
		metrics := make([]v1.Metric, len(config.MetricsConfig))
		var errorSlice []error

		for ind, metricConfig := range config.MetricsConfig {
			regex, err := regexp.Compile(metricConfig.Name + ".*([0-9]+ | [0-9]+.[0-9]+)")
			if err != nil {
				return nextCollectionTime, nil, fmt.Errorf("Metric %v cannot be found", metricConfig.Name)
			}
			matchString := regex.FindStringSubmatch(pageContent)
			if matchString != nil {
				regVal, err := strconv.ParseFloat(strings.TrimSpace(matchString[1]), 64)
				if err != nil {
					errorSlice = append(errorSlice, err)
				}
				metrics[ind].FloatPoints = []v1.FloatPoint{
					{Value: regVal, Timestamp: currentTime},
				}
			} else {
				errorSlice = append(errorSlice, fmt.Errorf("No match found for metric '%v' in config", metricConfig.Name))
			}

			metrics[ind].Name = metricConfig.Name
			metrics[ind].Type = v1.MetricGauge
		}

		return nextCollectionTime, metrics, compileErrors(errorSlice)

	default:
		return time.Now(), nil, fmt.Errorf("No support for %v", collector.Name)
	}
}
