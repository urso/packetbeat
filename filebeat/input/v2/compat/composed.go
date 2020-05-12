// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package compat

import (
	v2 "github.com/elastic/beats/v7/filebeat/input/v2"
	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/cfgfile"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/urso/sderr"
)

type composeFactory struct {
	factory  cfgfile.RunnerFactory
	fallback cfgfile.RunnerFactory
}

var _ cfgfile.RunnerFactory = composeFactory{}

func Combine(factory, fallback cfgfile.RunnerFactory) cfgfile.RunnerFactory {
	return composeFactory{factory: factory, fallback: fallback}
}

func (f composeFactory) CheckConfig(cfg *common.Config) error {
	err := f.factory.CheckConfig(cfg)
	if err == nil {
		return nil
	}

	return f.fallback.CheckConfig(cfg)
}

func (f composeFactory) Create(
	p beat.PipelineConnector,
	config *common.Config,
	meta *common.MapStrPointer,
) (cfgfile.Runner, error) {
	var runner cfgfile.Runner
	var err1, err2 error

	runner, err1 = f.factory.Create(p, config, meta)
	if err1 == nil {
		return runner, nil
	}

	runner, err2 = f.fallback.Create(p, config, meta)
	if err2 == nil {
		return runner, nil
	}

	// return err2 only if err1 indicates that the input type is not known to f.factory
	if sderr.Is(err1, v2.ErrUnknown) {
		return nil, err2
	}
	return nil, err1
}
