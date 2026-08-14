package service

type SolakonOne struct {
	// BatteryPower    float64 `json:"battery_power"`
	BatteryCapacity   float64 `json:"battery_capacity"`
	BatteryEfficiency float64 `json:"battery_efficiency"`
	ACMaxAllowedPower float64 `json:"ac_max_allowed_power"`
	Initials          SolakonOneInitialValues
}

type SolakonOneInitialValues struct {
	BatteryPower float64 `json:"battery_power"`
	BatteryLevel float64 `json:"battery_level"`
}
