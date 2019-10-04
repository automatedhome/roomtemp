package types

type ScheduleCell struct {
	From        string  `json:"from"`
	To          string  `json:"to"`
	Temperature float64 `json:"temperature"`
}

type Schedule struct {
	Workday            []ScheduleCell `json:"workday"`
	Freeday            []ScheduleCell `json:"freeday"`
	DefaultTemperature float64        `json:"defaultTemperature"`
}
