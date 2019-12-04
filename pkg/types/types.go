package types

import (
	common "github.com/automatedhome/common/pkg/types"
)

type Settings struct {
	Holiday  common.BoolPoint `yaml:"holiday"`
	Override common.DataPoint `yaml:"override"`
	Mode     struct {
		Value   string `yaml:"value,omitempty"`
		Address string `yaml:"address"`
	} `yaml:"mode"`
}

type Actuators struct {
	Expected common.DataPoint `yaml:"expected"`
}

type Config struct {
	Actuators Actuators `yaml:"actuators"`
	Settings  Settings  `yaml:"settings"`
	Schedule  string    `yaml:"scheduleTopic"`
}
