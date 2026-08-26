package service

type HANumberMode string
type HAStateClass string
type HADeviceClass string
type HAEntityCategory string
type BMSEntityID string

const (
	HANumberModeBoxed  HANumberMode = "box"
	HANumberModeSlider HANumberMode = "slider"
)

const (
	BatteryDuration        BMSEntityID = "sensor.go_bms_battery_duration"
	BMS                    BMSEntityID = "switch.go_bms_bms"
	BMSSimulation          BMSEntityID = "switch.go_bms_bms_simulation"
	MinumumBattery         BMSEntityID = "number.go_bms_minimum_battery"
	MinumumBatteryTime     BMSEntityID = "time.go_bms_minimum_battery_time"
	MaximumDrainForMinimum BMSEntityID = "sensor.go_bms_maximum_drain_for_minimum"
	EnergyCostForecast     BMSEntityID = "sensor.go_bms_energy_cost_forecast"
	EnergyCostCurrent      BMSEntityID = "sensor.go_bms_energy_cost_current"
)

const (
	HAStateClassMeasurement     HAStateClass = "measurement"
	HAStateClassTotal           HAStateClass = "total"
	HAStateClassTotalIncreasing HAStateClass = "total_increasing"
)

const (
	HADeviceClassBattery  HADeviceClass = "battery"
	HADeviceClassPower    HADeviceClass = "power"
	HADeviceClassEnergy   HADeviceClass = "energy"
	HADeviceClassDuration HADeviceClass = "duration"
	HADeviceClassMonetary HADeviceClass = "monetary"
)

const (
	HAEntityCategoryConfig     HAEntityCategory = "config"
	HAEntityCategoryDiagnostic HAEntityCategory = "diagnostic"
	HAEntityCategoryDefault    HAEntityCategory = ""
)

// Device metadata ties multiple entities to one device entry in HA
type HADevice struct {
	Identifiers   []string `json:"identifiers"`
	Name          string   `json:"name"`
	Model         string   `json:"model"`
	Manufacturer  string   `json:"manufacturer"`
	SwVersion     string   `json:"sw_version,omitempty"`
	SuggestedArea string   `json:"suggested_area,omitempty"`
}

// Sensor Discovery Configuration Payload
type HASensorConfig struct {
	Name                   string        `json:"name"`
	UniqueDataID           string        `json:"unique_id"`
	StateTopic             string        `json:"state_topic"`
	UnitOfMeasurement      string        `json:"unit_of_measurement,omitempty"`
	DeviceClass            HADeviceClass `json:"device_class,omitempty"`
	StateClass             HAStateClass  `json:"state_class,omitempty"`
	ValueTemplate          string        `json:"value_template,omitempty"`
	JSONAttributesTopic    string        `json:"json_attributes_topic,omitempty"`
	JSONAttributesTemplate string        `json:"json_attributes_template,omitempty"`
	Icon                   string        `json:"icon,omitempty"`
	FriendlyName           string        `json:"friendly_name,omitempty"`
	Device                 HADevice      `json:"device"`
}

type HASwitchConfig struct {
	Name          string   `json:"name"`
	UniqueDataID  string   `json:"unique_id"`
	StateTopic    string   `json:"state_topic"`
	CommandTopic  string   `json:"command_topic"`
	ValueTemplate string   `json:"value_template,omitempty"`
	PayloadOn     string   `json:"payload_on,omitempty"`
	PayloadOff    string   `json:"payload_off,omitempty"`
	StateOn       string   `json:"state_on,omitempty"`
	StateOff      string   `json:"state_off,omitempty"`
	Icon          string   `json:"icon,omitempty"`
	Device        HADevice `json:"device"`
}

type HANumberConfig struct {
	Name              string           `json:"name"`
	UniqueDataID      string           `json:"unique_id"`
	StateTopic        string           `json:"state_topic"`
	CommandTopic      string           `json:"command_topic"`
	ValueTemplate     string           `json:"value_template,omitempty"`
	Min               float64          `json:"min,omitempty"`
	Max               float64          `json:"max,omitempty"`
	Step              float64          `json:"step,omitempty"`
	Mode              HANumberMode     `json:"mode,omitempty"`
	Icon              string           `json:"icon,omitempty"`
	FriendlyName      string           `json:"friendly_name,omitempty"`
	UnitOfMeasurement string           `json:"unit_of_measurement,omitempty"`
	EntityCategory    HAEntityCategory `json:"entity_category,omitempty"`
	Device            HADevice         `json:"device"`
}

type HATimeConfig struct {
	Name              string           `json:"name"`
	UniqueDataID      string           `json:"unique_id"`
	StateTopic        string           `json:"state_topic"`
	CommandTopic      string           `json:"command_topic"`
	Icon              string           `json:"icon,omitempty"`
	FriendlyName      string           `json:"friendly_name,omitempty"`
	UnitOfMeasurement string           `json:"unit_of_measurement,omitempty"`
	EntityCategory    HAEntityCategory `json:"entity_category,omitempty"`
	ValueTemplate     string           `json:"value_template,omitempty"`
	Device            HADevice         `json:"device"`
}

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

type BatteryManagement struct {
	BMSEnabled           bool    `json:"bms_enabled"`
	BMSSimulationEnabled bool    `json:"bms_simulation_enabled"`
	MinimumBatteryLevel  float64 `json:"minimum_battery"`
	MinimumBatteryTime   string  `json:"minimum_battery_time"`
}
