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

package ilm

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/common/fmtstr"
)

type Config struct {
	Mode          Mode                     `config:"enabled"`
	Name          fmtstr.EventFormatString `config:"name"`
	RolloverAlias string                   `config:"rollover_alias"`
	Pattern       string                   `config:"pattern"`
	PolicyFile    string                   `config:"policy.file"`

	// CheckExists can disable the check for an existing policy. Check required
	// read_ilm privileges.  If check is disabled the policy will only be
	// installed if Overwrite is enabled.
	CheckExists bool `config:"check_exists"`

	// Enable always overwrite policy mode. This required manage_ilm privileges.
	Overwrite bool `config:"overwrite"`
}

//Mode is used for enumerating the ilm mode.
type Mode uint8

const (
	//ModeAuto enum 'auto'
	ModeAuto Mode = iota

	//ModeEnabled enum 'true'
	ModeEnabled

	//ModeDisabled enum 'false'
	ModeDisabled
)

const ilmDefaultPattern = "{now/d}-000001"

var ilmDefaultPolicy = common.MapStr{
	"policy": common.MapStr{
		"phases": common.MapStr{
			"hot": common.MapStr{
				"actions": common.MapStr{
					"rollover": common.MapStr{
						"max_size": "50gb",
						"max_age":  "30d",
					},
				},
			},
		},
	},
}

//Unpack creates enumeration value true, false or auto
func (m *Mode) Unpack(in string) error {
	in = strings.ToLower(in)

	if in == "auto" {
		*m = ModeAuto
		return nil
	}

	b, err := strconv.ParseBool(in)
	if err != nil {
		return fmt.Errorf("ilm.enabled` mode '%v' is invalid (try auto, true, false)", in)
	}

	if b {
		*m = ModeEnabled
	} else {
		*m = ModeDisabled
	}
	return nil
}

//Validate verifies that expected config options are given and valid
func (cfg *Config) Validate() error {
	if cfg.RolloverAlias == "" && cfg.Mode != ModeDisabled {
		return fmt.Errorf("rollover_alias must be set when ILM is not disabled")
	}
	return nil
}

func defaultConfig(info beat.Info) Config {
	name := fmt.Sprintf("%s-%s", info.Beat, info.Version)
	nameFmt := fmtstr.MustCompileEvent(name)

	return Config{
		Mode:          ModeAuto,
		Name:          *nameFmt,
		RolloverAlias: name,
		Pattern:       ilmDefaultPattern,
		PolicyFile:    "",
	}
}
