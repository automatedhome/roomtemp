package types

import (
	common "github.com/automatedhome/common/pkg/types"
)

type Sensors struct {
	Holiday  common.BoolPoint `yaml:"holiday"`
	Override common.DataPoint `yaml:"override"`
}

type Actuators struct {
	Expected common.DataPoint `yaml:"expected"`
}

type Config struct {
	Actuators Actuators `yaml:"actuators"`
	Sensors   Sensors   `yaml:"sensors"`
	Schedule  string    `yaml:"scheduleTopic"`
}
