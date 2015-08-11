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
	//	"fmt"
	"io/ioutil"
	//	"net/http"
	//	"net/http/httptest"
	"testing"

	"github.com/google/cadvisor/info/v1"
	"github.com/stretchr/testify/assert"
)

func TestPrometheus(t *testing.T) {
	assert := assert.New(t)

	//Create a prometheus collector using the config file 'sample_config_prometheus.json'
	configFile, err := ioutil.ReadFile("config/sample_config_prometheus.json")
	collector, err := NewPrometheusCollector("Prometheus", configFile)
	assert.NoError(err)
	assert.Equal(collector.name, "Prometheus")
	assert.Equal(collector.configFile.Endpoint, "http://localhost:8080/metrics")

	var specs []v1.MetricSpec
	//	if len(collector.configFile.MetricsConfig) == 0 {
	//		specs = collector.GetSpec(true)
	//	} else {
	specs = collector.GetSpec()
	//	}
	assert.NotNil(specs)
	metrics := map[string][]v1.MetricVal{}
	_, _, errMetric := collector.Collect(metrics)
	assert.NoError(errMetric)
}
