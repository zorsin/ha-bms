package service

import (
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type HANumberMode string
type HAStateClass string
type HADeviceClass string
type HAEntityCategory string
type BMSEntityID string
type ControlModeOpt string
type AutoModeOpt string
type SolakonForceMode string

const (
	HANumberModeBoxed  HANumberMode = "box"
	HANumberModeSlider HANumberMode = "slider"
)

const (
	BatteryDuration            BMSEntityID = "sensor.go_bms_battery_duration"
	BMS                        BMSEntityID = "switch.go_bms_bms"
	BMSSimulation              BMSEntityID = "switch.go_bms_bms_simulation"
	MinumumBattery             BMSEntityID = "number.go_bms_minimum_battery"
	MinumumBatteryTime         BMSEntityID = "time.go_bms_minimum_battery_time"
	MaximumDrainForMinimum     BMSEntityID = "sensor.go_bms_maximum_drain_for_minimum"
	EnergyCostCurrent          BMSEntityID = "sensor.go_bms_energy_cost_current"
	EnergyCostForecast         BMSEntityID = "sensor.go_bms_energy_cost_forecast"
	EnergyCostForecastTomorrow BMSEntityID = "sensor.go_bms_energy_cost_forecast_tomorrow"

	// control
	ControlDuration BMSEntityID = "number.go_bms_control_duration"
	ControlTime     BMSEntityID = "sensor.go_bms_control_time"
	ControlPower    BMSEntityID = "number.go_bms_control_power"
	ControlMode     BMSEntityID = "select.go_bms_control_mode"
	AutoMode        BMSEntityID = "select.go_bms_auto_mode"

	// simulation entities for testing
	SimForceDuration BMSEntityID = "number.go_bms_sim_force_duration"
	SimForceMode     BMSEntityID = "select.go_bms_sim_force_mode"
	SimForcePower    BMSEntityID = "number.go_bms_sim_force_power"
)

const (
	ControlModeOptOff       ControlModeOpt = "off"
	ControlModeOptDischarge ControlModeOpt = "discharge"
	ControlModeOptCharge    ControlModeOpt = "charge"
	ControlModeOptAuto      ControlModeOpt = "auto"

	AutoModeOptOff              AutoModeOpt = "off"
	AutoModeOptChargeWhenNeeded AutoModeOpt = "charge_when_needed"
	AutoModeOptForceTargetDrain AutoModeOpt = "force_target_drain"
)

const (
	HAStateClassMeasurement     HAStateClass = "measurement"
	HAStateClassTotal           HAStateClass = "total"
	HAStateClassTotalIncreasing HAStateClass = "total_increasing"
)

const (
	HADeviceClassBattery   HADeviceClass = "battery"
	HADeviceClassPower     HADeviceClass = "power"
	HADeviceClassEnergy    HADeviceClass = "energy"
	HADeviceClassDuration  HADeviceClass = "duration"
	HADeviceClassMonetary  HADeviceClass = "monetary"
	HADeviceClassTimestamp HADeviceClass = "timestamp"
)

const (
	HAEntityCategoryConfig     HAEntityCategory = "config"
	HAEntityCategoryDiagnostic HAEntityCategory = "diagnostic"
	HAEntityCategoryDefault    HAEntityCategory = ""
)

var mapperForceModeToValue = map[ControlModeOpt]SolakonForceMode{
	ControlModeOptOff:       SolakonForceModeOff,
	ControlModeOptDischarge: SolakonForceModeDischarge,
	ControlModeOptCharge:    SolakonForceModeCharge,
}

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
	DeviceClass       HADeviceClass    `json:"device_class,omitempty"`
	StateClass        HAStateClass     `json:"state_class,omitempty"`
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

type HASelectConfig struct {
	Name            string           `json:"name"`
	UniqueDataID    string           `json:"unique_id"`
	StateTopic      string           `json:"state_topic"`
	CommandTopic    string           `json:"command_topic"`
	Icon            string           `json:"icon,omitempty"`
	FriendlyName    string           `json:"friendly_name,omitempty"`
	Options         []string         `json:"options"`
	ValueTemplate   string           `json:"value_template,omitempty"`
	CommandTemplate string           `json:"command_template,omitempty"`
	EntityCategory  HAEntityCategory `json:"entity_category,omitempty"`
	Device          HADevice         `json:"device"`
}

type HAEntityConfig struct {
	configTopic string
	config      any // HASensorConfig or HASwitchConfig
	subTopic    string
	subCallback mqtt.MessageHandler
}

type ForcastPoint struct {
	DateTime string `json:"datetime"`
	Value    string `json:"value"`
}

type ForcastData struct {
	Current  string          `json:"current"`
	Forecast []*ForcastPoint `json:"forecast"`
}

type HAStatePayload struct {
	BMSEnabled                 string       `json:"bms_enabled,omitempty"`
	BMSSimulationEnabled       string       `json:"bms_simulation_enabled,omitempty"`
	BatteryDuration            string       `json:"battery_duration,omitempty"`
	MinimumBattery             string       `json:"minimum_battery,omitempty"`
	MinimumBatteryTime         string       `json:"minimum_battery_time,omitempty"`
	MaximumDrainForMinimum     string       `json:"maximum_drain_for_minimum,omitempty"`
	EnergyCostCurrent          string       `json:"energy_cost_current,omitempty"`
	EnergyCostForecast         *ForcastData `json:"energy_cost_forecast,omitempty"`
	EnergyCostForecastTomorrow *ForcastData `json:"energy_cost_forecast_tomorrow,omitempty"`
	ControlDuration            string       `json:"control_duration,omitempty"`
	ControlTime                string       `json:"control_time,omitempty"`
	ControlPower               string       `json:"control_power,omitempty"`
	ControlMode                string       `json:"control_mode,omitempty"`
	SimForceDuration           string       `json:"sim_force_duration,omitempty"`
	SimForceMode               string       `json:"sim_force_mode,omitempty"`
	SimForcePower              string       `json:"sim_force_power,omitempty"`
	AutoMode                   string       `json:"auto_mode,omitempty"`
}

const (
	SolakonForceModeOff       SolakonForceMode = "0"
	SolakonForceModeDischarge SolakonForceMode = "1"
	SolakonForceModeCharge    SolakonForceMode = "3"
)

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
	mux                  *sync.RWMutex
	BMSEnabled           bool           `json:"bms_enabled"`
	BMSSimulationEnabled bool           `json:"bms_simulation_enabled"`
	MinimumBatteryLevel  float64        `json:"minimum_battery"`
	MinimumBatteryTime   string         `json:"minimum_battery_time"`
	ControlDuration      time.Duration  `json:"control_duration"`
	ControlTime          *time.Time     `json:"control_time"`
	ControlPower         float64        `json:"control_power"`
	ControlMode          ControlModeOpt `json:"control_mode"`
	AutoMode             AutoModeOpt    `json:"auto_mode"`
}

func (bm *BatteryManagement) SetBMSEnabled(enabled bool) {
	bm.mux.Lock()
	defer bm.mux.Unlock()
	bm.BMSEnabled = enabled
}

func (bm *BatteryManagement) SetBMSSimulationEnabled(enabled bool) {
	bm.mux.Lock()
	defer bm.mux.Unlock()
	bm.BMSSimulationEnabled = enabled
}

func (bm *BatteryManagement) GetBMSEnabled() bool {
	bm.mux.RLock()
	defer bm.mux.RUnlock()
	return bm.BMSEnabled
}

func (bm *BatteryManagement) GetBMSSimulationEnabled() bool {
	bm.mux.RLock()
	defer bm.mux.RUnlock()
	return bm.BMSSimulationEnabled
}
