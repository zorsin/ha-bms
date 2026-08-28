package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const (
	mqttBMSStateTopic            = "go_bms/state"
	mqttBMSStateTopicPlaceholder = "go_bms/state/%s"
)

func (s *service) initMQTTClient(ctx context.Context) error {
	opts := mqtt.NewClientOptions().AddBroker(s.cfg.HAMQTTURL)
	opts.SetClientID("go_service_publisher")
	opts.SetUsername(s.cfg.HAMQTTUser)
	opts.SetPassword(s.cfg.HAMQTTPassword)
	client := mqtt.NewClient(opts)

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		s.logger.ErrorContext(ctx, "Failed to connect to MQTT", slog.Any("err", token.Error()))
		return token.Error()
	}
	s.mqttClient = client
	return nil
}

func (s *service) createEntities(device HADevice) map[BMSEntityID]HAEntityConfig {
	entities := make(map[BMSEntityID]HAEntityConfig)

	entities[BatteryDuration] = HAEntityConfig{
		configTopic: "homeassistant/sensor/go_bms/battery_duration_001/config",
		config: HASensorConfig{
			FriendlyName:      "Battery Duration",
			Name:              "battery_duration",
			UniqueDataID:      "battery_duration_001",
			StateTopic:        fmt.Sprintf(mqttBMSStateTopicPlaceholder, "battery_duration"),
			ValueTemplate:     "{{ value_json.battery_duration }}",
			UnitOfMeasurement: "h",
			DeviceClass:       HADeviceClassDuration,
			StateClass:        HAStateClassMeasurement,
			Device:            device,
		},
	}

	entities[BMS] = HAEntityConfig{
		configTopic: "homeassistant/switch/go_bms/bms_switch_001/config",
		config: HASwitchConfig{
			Name:          "bms",
			UniqueDataID:  "bms_switch_001",
			StateTopic:    fmt.Sprintf(mqttBMSStateTopicPlaceholder, "bms_enabled"),
			CommandTopic:  "go_bms/bms/set",
			ValueTemplate: "{{ value_json.bms_enabled }}",
			PayloadOn:     "true",
			PayloadOff:    "false",
			StateOn:       "true",
			StateOff:      "false",
			Device:        device,
		},
		subTopic: "go_bms/bms/set",
		subCallback: func(c mqtt.Client, m mqtt.Message) {
			commandPayload := string(m.Payload())
			s.broad.Publish(StateUpdate{ID: string(BMS), Value: commandPayload})
		},
	}

	entities[BMSSimulation] = HAEntityConfig{
		configTopic: "homeassistant/switch/go_bms/bms_simulation_switch_001/config",
		config: HASwitchConfig{
			Name:          "bms_simulation",
			UniqueDataID:  "bms_simulation_switch_001",
			StateTopic:    fmt.Sprintf(mqttBMSStateTopicPlaceholder, "bms_simulation_enabled"),
			CommandTopic:  "go_bms/bms_simulation/set",
			ValueTemplate: "{{ value_json.bms_simulation_enabled }}",
			PayloadOn:     "true",
			PayloadOff:    "false",
			StateOn:       "true",
			StateOff:      "false",
			Device:        device,
		},
		subTopic: "go_bms/bms_simulation/set",
		subCallback: func(c mqtt.Client, m mqtt.Message) {
			commandPayload := string(m.Payload())
			s.broad.Publish(StateUpdate{ID: string(BMSSimulation), Value: commandPayload})
		},
	}

	entities[MinumumBattery] = HAEntityConfig{
		configTopic: "homeassistant/number/go_bms/minimum_battery_001/config",
		config: HANumberConfig{
			FriendlyName:      "Minimum Battery",
			Name:              "minimum_battery",
			UniqueDataID:      "minimum_battery_001",
			StateTopic:        fmt.Sprintf(mqttBMSStateTopicPlaceholder, "minimum_battery"),
			ValueTemplate:     "{{ value_json.minimum_battery }}",
			UnitOfMeasurement: "%",
			Min:               10,
			Max:               100,
			Step:              1,
			Mode:              HANumberModeSlider,
			EntityCategory:    HAEntityCategoryConfig,
			CommandTopic:      "go_bms/minimum_battery/set",
			// DeviceClass:       "",
			// StateClass: HAStateClassMeasurement,
			Device: device,
		},
		subTopic: "go_bms/minimum_battery/set",
		subCallback: func(c mqtt.Client, m mqtt.Message) {
			commandPayload := string(m.Payload())
			s.broad.Publish(StateUpdate{ID: string(MinumumBattery), Value: commandPayload})
		},
	}

	entities[MinumumBatteryTime] = HAEntityConfig{
		configTopic: "homeassistant/time/go_bms/minimum_battery_time_001/config",
		config: HATimeConfig{
			FriendlyName:   "Minimum Battery Time",
			Name:           "minimum_battery_time",
			UniqueDataID:   "minimum_battery_time_001",
			StateTopic:     fmt.Sprintf(mqttBMSStateTopicPlaceholder, "minimum_battery_time"),
			ValueTemplate:  "{{ value_json.minimum_battery_time }}",
			CommandTopic:   "go_bms/minimum_battery_time/set",
			EntityCategory: HAEntityCategoryConfig,
			Device:         device,
			Icon:           "mdi:clock-outline",
		},
		subTopic: "go_bms/minimum_battery_time/set",
		subCallback: func(c mqtt.Client, m mqtt.Message) {
			commandPayload := string(m.Payload())
			s.broad.Publish(StateUpdate{ID: string(MinumumBatteryTime), Value: commandPayload})
		},
	}

	entities[MaximumDrainForMinimum] = HAEntityConfig{
		configTopic: "homeassistant/sensor/go_bms/maximum_drain_for_minimum_001/config",
		config: HASensorConfig{
			FriendlyName:      "Maximum Drain for Minimum",
			Name:              "maximum_drain_for_minimum",
			UniqueDataID:      "maximum_drain_for_minimum_001",
			StateTopic:        fmt.Sprintf(mqttBMSStateTopicPlaceholder, "maximum_drain_for_minimum"),
			ValueTemplate:     "{{ value_json.maximum_drain_for_minimum }}",
			UnitOfMeasurement: "W",
			Icon:              "mdi:flash-alert-outline",
			DeviceClass:       HADeviceClassPower,
			StateClass:        HAStateClassMeasurement,
			Device:            device,
		},
	}

	entities[EnergyCostCurrent] = HAEntityConfig{
		configTopic: "homeassistant/sensor/go_bms/energy_cost_current_001/config",
		config: HASensorConfig{
			FriendlyName:      "Energy Cost Current",
			Name:              "energy_cost_current",
			UniqueDataID:      "energy_cost_current_001",
			StateTopic:        fmt.Sprintf(mqttBMSStateTopicPlaceholder, "energy_cost_current"),
			ValueTemplate:     "{{ value_json.energy_cost_current }}",
			UnitOfMeasurement: "EUR/kWh",
			Icon:              "mdi:cash",
			DeviceClass:       HADeviceClassMonetary,
			StateClass:        HAStateClassMeasurement,
			Device:            device,
		},
	}

	entities[EnergyCostForecast] = HAEntityConfig{
		configTopic: "homeassistant/sensor/go_bms/energy_cost_forecast_001/config",
		config: HASensorConfig{
			FriendlyName:           "Energy Cost Forecast",
			Name:                   "energy_cost_forecast",
			UniqueDataID:           "energy_cost_forecast_001",
			StateTopic:             fmt.Sprintf(mqttBMSStateTopicPlaceholder, "energy_cost_forecast"),
			ValueTemplate:          "{{ value_json.energy_cost_forecast.current }}",
			JSONAttributesTopic:    fmt.Sprintf(mqttBMSStateTopicPlaceholder, "energy_cost_forecast"),
			JSONAttributesTemplate: "{% if value_json.energy_cost_forecast is defined %}{{ {'forecast': value_json.energy_cost_forecast.forecast} | tojson }}{% endif %}",
			UnitOfMeasurement:      "EUR/kWh",
			Icon:                   "mdi:cash",
			DeviceClass:            HADeviceClassMonetary,
			StateClass:             HAStateClassMeasurement,
			Device:                 device,
		},
	}

	entities[EnergyCostForecastTomorrow] = HAEntityConfig{
		configTopic: "homeassistant/sensor/go_bms/energy_cost_forecast_tomorrow_001/config",
		config: HASensorConfig{
			FriendlyName:           "Energy Cost Forecast Tomorrow",
			Name:                   "energy_cost_forecast_tomorrow",
			UniqueDataID:           "energy_cost_forecast_tomorrow_001",
			StateTopic:             fmt.Sprintf(mqttBMSStateTopicPlaceholder, "energy_cost_forecast_tomorrow"),
			ValueTemplate:          "{{ value_json.energy_cost_forecast_tomorrow.current }}",
			JSONAttributesTopic:    fmt.Sprintf(mqttBMSStateTopicPlaceholder, "energy_cost_forecast_tomorrow"),
			JSONAttributesTemplate: "{% if value_json.energy_cost_forecast_tomorrow is defined %}{{ {'forecast': value_json.energy_cost_forecast_tomorrow.forecast} | tojson }}{% endif %}",
			UnitOfMeasurement:      "EUR/kWh",
			Icon:                   "mdi:cash",
			DeviceClass:            HADeviceClassMonetary,
			StateClass:             HAStateClassMeasurement,
			Device:                 device,
		},
	}

	entities[ControlDuration] = HAEntityConfig{
		configTopic: "homeassistant/number/go_bms/control_duration_001/config",
		config: HANumberConfig{
			FriendlyName:      "Control Duration",
			Name:              "control_duration",
			UniqueDataID:      "control_duration_001",
			StateTopic:        fmt.Sprintf(mqttBMSStateTopicPlaceholder, "control_duration"),
			ValueTemplate:     "{{ value_json.control_duration }}",
			UnitOfMeasurement: "min",
			Min:               0,
			Max:               60,
			Step:              5,
			Mode:              HANumberModeSlider,
			EntityCategory:    HAEntityCategoryConfig,
			CommandTopic:      "go_bms/control_duration/set",
			DeviceClass:       HADeviceClassDuration,
			StateClass:        HAStateClassMeasurement,
			Device:            device,
		},
		subTopic: "go_bms/control_duration/set",
		subCallback: func(c mqtt.Client, m mqtt.Message) {
			commandPayload := string(m.Payload())
			s.broad.Publish(StateUpdate{ID: string(ControlDuration), Value: commandPayload})
		},
	}

	entities[ControlTime] = HAEntityConfig{
		configTopic: "homeassistant/sensor/go_bms/control_time_001/config",
		config: HASensorConfig{
			FriendlyName:  "Control Time",
			Name:          "control_time",
			UniqueDataID:  "control_time_001",
			StateTopic:    fmt.Sprintf(mqttBMSStateTopicPlaceholder, "control_time"),
			ValueTemplate: "{{ value_json.control_time }}",
			// UnitOfMeasurement: "min",
			DeviceClass: HADeviceClassTimestamp,
			StateClass:  HAStateClassMeasurement,
			Device:      device,
		},
	}

	entities[ControlMode] = HAEntityConfig{
		configTopic: "homeassistant/select/go_bms/control_mode_001/config",
		config: HASelectConfig{
			FriendlyName:  "Control Mode",
			Name:          "control_mode",
			UniqueDataID:  "control_mode_001",
			StateTopic:    fmt.Sprintf(mqttBMSStateTopicPlaceholder, "control_mode"),
			ValueTemplate: "{{ value_json.control_mode}}",
			Options: []string{
				string(ControlModeOptOff),
				string(ControlModeOptDischarge),
				string(ControlModeOptCharge),
				string(ControlModeOptAuto),
			},
			EntityCategory: HAEntityCategoryConfig,
			Icon:           "mdi:flash",
			CommandTopic:   "go_bms/control_mode/set",
			Device:         device,
		},
		subTopic: "go_bms/control_mode/set",
		subCallback: func(c mqtt.Client, m mqtt.Message) {
			commandPayload := string(m.Payload())
			s.broad.Publish(StateUpdate{ID: string(ControlMode), Value: commandPayload})
		},
	}

	entities[ControlPower] = HAEntityConfig{
		configTopic: "homeassistant/number/go_bms/control_power_001/config",
		config: HANumberConfig{
			FriendlyName:      "Control Power",
			Name:              "control_power",
			UniqueDataID:      "control_power_001",
			StateTopic:        fmt.Sprintf(mqttBMSStateTopicPlaceholder, "control_power"),
			ValueTemplate:     "{{ value_json.control_power }}",
			UnitOfMeasurement: "W",
			Min:               0,
			Max:               1200,
			Step:              10,
			Mode:              HANumberModeBoxed,
			EntityCategory:    HAEntityCategoryConfig,
			Icon:              "mdi:flash",
			CommandTopic:      "go_bms/control_power/set",
			DeviceClass:       HADeviceClassPower,
			StateClass:        HAStateClassMeasurement,
			Device:            device,
		},
		subTopic: "go_bms/control_power/set",
		subCallback: func(c mqtt.Client, m mqtt.Message) {
			commandPayload := string(m.Payload())
			s.broad.Publish(StateUpdate{ID: string(ControlPower), Value: commandPayload})
		},
	}

	entities[SimForceDuration] = HAEntityConfig{
		configTopic: "homeassistant/number/go_bms/sim_force_duration_001/config",
		config: HANumberConfig{
			FriendlyName:      "Sim Force Duration",
			Name:              "sim_force_duration",
			UniqueDataID:      "sim_force_duration_001",
			StateTopic:        fmt.Sprintf(mqttBMSStateTopicPlaceholder, "sim_force_duration"),
			ValueTemplate:     "{{ value_json.sim_force_duration }}",
			UnitOfMeasurement: "min",
			Min:               0,
			Max:               120,
			Step:              1,
			Mode:              HANumberModeBoxed,
			EntityCategory:    HAEntityCategoryDiagnostic,
			CommandTopic:      "go_bms/sim_force_duration/set",
			DeviceClass:       HADeviceClassDuration,
			StateClass:        HAStateClassMeasurement,
			Device:            device,
		},
		subTopic: "go_bms/sim_force_duration/set",
		subCallback: func(c mqtt.Client, m mqtt.Message) {
			commandPayload := string(m.Payload())
			s.broad.Publish(StateUpdate{ID: string(SimForceDuration), Value: commandPayload})
		},
	}

	entities[SimForcePower] = HAEntityConfig{
		configTopic: "homeassistant/number/go_bms/sim_force_power_001/config",
		config: HANumberConfig{
			FriendlyName:      "Sim Force Power",
			Name:              "sim_force_power",
			UniqueDataID:      "sim_force_power_001",
			StateTopic:        fmt.Sprintf(mqttBMSStateTopicPlaceholder, "sim_force_power"),
			ValueTemplate:     "{{ value_json.sim_force_power }}",
			UnitOfMeasurement: "W",
			Min:               0,
			Max:               1500,
			Step:              1,
			Mode:              HANumberModeBoxed,
			EntityCategory:    HAEntityCategoryDiagnostic,
			Icon:              "mdi:flash",
			CommandTopic:      "go_bms/sim_force_power/set",
			DeviceClass:       HADeviceClassPower,
			StateClass:        HAStateClassMeasurement,

			Device: device,
		},
		subTopic: "go_bms/sim_force_power/set",
		subCallback: func(c mqtt.Client, m mqtt.Message) {
			commandPayload := string(m.Payload())
			s.broad.Publish(StateUpdate{ID: string(SimForcePower), Value: commandPayload})
		},
	}

	entities[SimForceMode] = HAEntityConfig{
		configTopic: "homeassistant/select/go_bms/sim_force_mode_001/config",
		config: HASelectConfig{
			FriendlyName:  "Sim Force Mode",
			Name:          "sim_force_mode",
			UniqueDataID:  "sim_force_mode_001",
			StateTopic:    fmt.Sprintf(mqttBMSStateTopicPlaceholder, "sim_force_mode"),
			ValueTemplate: "{{ value_json.sim_force_mode}}",
			Options: []string{
				string(ControlModeOptOff),
				string(ControlModeOptDischarge),
				string(ControlModeOptCharge),
			},
			EntityCategory: HAEntityCategoryDiagnostic,
			Icon:           "mdi:flash",
			CommandTopic:   "go_bms/sim_force_mode/set",
			// DeviceClass:       "",
			// StateClass:        HAStateClassMeasurement,
			Device: device,
		},
		subTopic: "go_bms/sim_force_mode/set",
		subCallback: func(c mqtt.Client, m mqtt.Message) {
			commandPayload := string(m.Payload())
			s.broad.Publish(StateUpdate{ID: string(SimForceMode), Value: commandPayload})
		},
	}

	entities[AutoMode] = HAEntityConfig{
		configTopic: "homeassistant/select/go_bms/auto_mode_001/config",
		config: HASelectConfig{
			FriendlyName:  "Auto Mode",
			Name:          "auto_mode",
			UniqueDataID:  "auto_mode_001",
			StateTopic:    fmt.Sprintf(mqttBMSStateTopicPlaceholder, "auto_mode"),
			ValueTemplate: "{{ value_json.auto_mode}}",
			Options: []string{
				string(AutoModeOptOff),
				string(AutoModeOptChargeWhenNeeded),
				string(AutoModeOptForceTargetDrain),
			},
			EntityCategory: HAEntityCategoryConfig,
			Icon:           "mdi:flash",
			CommandTopic:   "go_bms/auto_mode/set",
			Device:         device,
		},
		subTopic: "go_bms/auto_mode/set",
		subCallback: func(c mqtt.Client, m mqtt.Message) {
			commandPayload := string(m.Payload())

			s.broad.Publish(StateUpdate{ID: string(AutoMode), Value: commandPayload})
		},
	}

	return entities
}

func (s *service) initEntities(ctx context.Context) {
	myDevice := HADevice{
		Identifiers:  []string{"go_bms_service"}, // Unique device ID
		Name:         "Go BMS",
		Model:        "v1.0 BMS",
		Manufacturer: "Go BMS",
		SwVersion:    "1.0.0",
		// SuggestedArea: "BMS",
	}
	entities := s.createEntities(myDevice)

	// Publish Entity Config (Must be set to Retain: true)
	for key, entity := range entities {
		configBytes, _ := json.Marshal(entity.config)
		token := s.mqttClient.Publish(entity.configTopic, 0, true, configBytes)
		token.Wait()
		s.logger.DebugContext(ctx, "Published entity", slog.String("id", string(key)))
		if entity.subTopic != "" && entity.subCallback != nil {
			token = s.mqttClient.Subscribe(entity.subTopic, 0, entity.subCallback)
			token.Wait()
			s.logger.DebugContext(ctx, "Subscribed entity", slog.String("id", string(key)))
		}
	}

	// Set default state values for the entities
	go func() {
		bmsEnabled := "false"
		if s.batteryManagement.GetBMSEnabled() {
			bmsEnabled = "true"
		}
		bmsSimulationEnabled := "true"
		// if !s.batteryManagement.GetBMSSimulationEnabled() {
		// 	bmsSimulationEnabled = "false"
		// }

		now := time.Now()
		controlTimeStr := formatTimestamp(&now)
		if s.batteryManagement.ControlTime != nil {
			controlTimeStr = formatTimestamp(s.batteryManagement.ControlTime)
		}
		controlDurationStr := "0"
		if s.batteryManagement.ControlDuration > 0 {
			controlDurationStr = formatFloatDuration(s.batteryManagement.ControlDuration)
		}

		// set loaded values
		statePayload := HAStatePayload{
			BMSEnabled:           bmsEnabled,
			BMSSimulationEnabled: bmsSimulationEnabled,
			ControlTime:          controlTimeStr,
			ControlDuration:      controlDurationStr,
			ControlMode:          string(s.batteryManagement.ControlMode),
			ControlPower:         formatFloat(s.batteryManagement.ControlPower),
			AutoMode:             string(s.batteryManagement.AutoMode),
		}

		// set defaults
		if s.cfg.MQTTSetDefaults {
			statePayload.BatteryDuration = "0.0"
			statePayload.MinimumBattery = strconv.FormatFloat(s.batteryManagement.MinimumBatteryLevel, 'f', 2, 64)
			statePayload.MinimumBatteryTime = s.batteryManagement.MinimumBatteryTime
			statePayload.MaximumDrainForMinimum = "0.0"
			statePayload.EnergyCostForecast = &ForcastData{
				Current: "0.0",
				Forecast: []*ForcastPoint{
					{DateTime: time.Now().UTC().Format(time.RFC3339), Value: "0.001"},
				},
			}
			statePayload.EnergyCostForecastTomorrow = &ForcastData{
				Current: "0.0",
				Forecast: []*ForcastPoint{
					{DateTime: time.Now().Add(25 * time.Hour).UTC().Format(time.RFC3339), Value: "0.001"},
				},
			}
			statePayload.EnergyCostCurrent = "0.0"
			statePayload.SimForceDuration = "0.0"
			statePayload.SimForceMode = "off"
			statePayload.SimForcePower = "0.0"
		}
		time.Sleep(2 * time.Second) // Wait for entities to be registered in HA
		s.publishState(statePayload, true)
		s.logger.DebugContext(ctx, "Published initial state data")
	}()
}

func (s *service) publishState(state HAStatePayload, retained bool) {
	stateBytes, _ := json.Marshal(state)
	// get all attributes with values from state struct
	keys := getPopulatedJSONFields(state)

	for _, key := range keys {
		topic := fmt.Sprintf(mqttBMSStateTopicPlaceholder, key)
		s.logger.Debug("Updating state", slog.String("topic", topic), slog.String("state", string(stateBytes)))
		token := s.mqttClient.Publish(topic, 0, retained, stateBytes)
		token.Wait()
		if token.Error() != nil {
			s.logger.Error("Failed to publish state update", slog.Any("err", token.Error()))
		}
	}
}

func (s *service) updateState(state HAStatePayload, isSensor bool) {
	if s.batteryManagement.GetBMSEnabled() || !isSensor {
		s.publishState(state, false)
	}
}

func (s *service) manageStates(ctx context.Context) {
	chBatteryDuration := s.broad.Subscribe(string(BatteryDuration), 1)
	chBMSEnabled := s.broad.Subscribe(string(BMS), 1)
	chBMSSimulationEnabled := s.broad.Subscribe(string(BMSSimulation), 1)
	chMinimumBattery := s.broad.Subscribe(string(MinumumBattery), 1)
	chMinimumBatteryTime := s.broad.Subscribe(string(MinumumBatteryTime), 1)
	chMaximumDrainForMinimum := s.broad.Subscribe(string(MaximumDrainForMinimum), 1)
	chControlDuration := s.broad.Subscribe(string(ControlDuration), 1)
	chControlTime := s.broad.Subscribe(string(ControlTime), 1)
	chControlMode := s.broad.Subscribe(string(ControlMode), 1)
	chControlPower := s.broad.Subscribe(string(ControlPower), 1)
	chAutoMode := s.broad.Subscribe(string(AutoMode), 1)
	chSimForceDuration := s.broad.Subscribe(string(SimForceDuration), 1)
	chSimForcePower := s.broad.Subscribe(string(SimForcePower), 1)
	chSimForceMode := s.broad.Subscribe(string(SimForceMode), 1)

	for {
		select {
		case <-ctx.Done():
			return
		case v := <-chBatteryDuration:
			s.updateState(HAStatePayload{
				BatteryDuration: v.Value,
			}, true)
		case v := <-chBMSEnabled:
			s.batteryManagement.SetBMSEnabled(v.Value == "true")
			s.updateState(HAStatePayload{
				BMSEnabled: v.Value,
			}, false)
		case v := <-chBMSSimulationEnabled:
			s.batteryManagement.SetBMSSimulationEnabled(v.Value == "true")
			s.updateState(HAStatePayload{
				BMSSimulationEnabled: v.Value,
			}, false)
		case v := <-chMinimumBattery:
			s.updateState(HAStatePayload{
				MinimumBattery: v.Value,
			}, false)
		case v := <-chMinimumBatteryTime:
			s.updateState(HAStatePayload{
				MinimumBatteryTime: v.Value,
			}, false)
		case v := <-chMaximumDrainForMinimum:
			s.updateState(HAStatePayload{
				MaximumDrainForMinimum: v.Value,
			}, true)
		case v := <-chControlDuration:
			s.batteryManagement.ControlDuration = parseFloatDuration(v.Value)
			s.updateState(HAStatePayload{
				ControlDuration: v.Value,
			}, false)
		case v := <-chControlTime:
			if ct, ok := parseTimestamp(v.Value); ok {
				s.batteryManagement.ControlTime = ct
				s.updateState(HAStatePayload{
					ControlTime: v.Value,
				}, false)
			}
		case v := <-chControlMode:
			s.batteryManagement.ControlMode = ControlModeOpt(v.Value)
			s.updateState(HAStatePayload{
				ControlMode: v.Value,
			}, false)
		case v := <-chControlPower:
			s.batteryManagement.ControlPower = parseFloat(v.Value)
			s.updateState(HAStatePayload{
				ControlPower: v.Value,
			}, false)
		case v := <-chAutoMode:
			s.batteryManagement.AutoMode = AutoModeOpt(v.Value)
			s.updateState(HAStatePayload{
				AutoMode: v.Value,
			}, false)
		case v := <-chSimForceDuration:
			s.updateState(HAStatePayload{
				SimForceDuration: v.Value,
			}, false)
		case v := <-chSimForcePower:
			s.updateState(HAStatePayload{
				SimForcePower: v.Value,
			}, false)
		case v := <-chSimForceMode:
			s.updateState(HAStatePayload{
				SimForceMode: v.Value,
			}, false)
		}
	}
}
