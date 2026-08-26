package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const (
	mqttBMSStateTopic = "go_bms/state"
)

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
}

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

type HAEntityConfig struct {
	configTopic string
	config      any // HASensorConfig or HASwitchConfig
	subTopic    string
	subCallback mqtt.MessageHandler
}

func (s *service) createEntities(device HADevice, b *Broadcaster) map[BMSEntityID]HAEntityConfig {
	entities := make(map[BMSEntityID]HAEntityConfig)

	entities[BatteryDuration] = HAEntityConfig{
		configTopic: "homeassistant/sensor/go_bms/battery_duration_001/config",
		config: HASensorConfig{
			FriendlyName:      "Battery Duration",
			Name:              "battery_duration",
			UniqueDataID:      "battery_duration_001",
			StateTopic:        mqttBMSStateTopic,
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
			StateTopic:    mqttBMSStateTopic,
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
			s.batteryManagement.BMSEnabled = commandPayload == "true"
			s.updateState(HAStatePayload{
				BMSEnabled: commandPayload,
			}, false)
			b.Publish(StateUpdate{ID: string(BMS), Value: commandPayload})
		},
	}

	entities[BMSSimulation] = HAEntityConfig{
		configTopic: "homeassistant/switch/go_bms/bms_simulation_switch_001/config",
		config: HASwitchConfig{
			Name:          "bms_simulation",
			UniqueDataID:  "bms_simulation_switch_001",
			StateTopic:    mqttBMSStateTopic,
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
			s.batteryManagement.BMSSimulationEnabled = commandPayload == "true"
			s.updateState(HAStatePayload{
				BMSSimulationEnabled: commandPayload,
			}, false)
			b.Publish(StateUpdate{ID: string(BMSSimulation), Value: commandPayload})
		},
	}

	entities[MinumumBattery] = HAEntityConfig{
		configTopic: "homeassistant/number/go_bms/minimum_battery_001/config",
		config: HANumberConfig{
			FriendlyName:      "Minimum Battery",
			Name:              "minimum_battery",
			UniqueDataID:      "minimum_battery_001",
			StateTopic:        mqttBMSStateTopic,
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
			b.Publish(StateUpdate{ID: string(MinumumBattery), Value: commandPayload})
			s.updateState(HAStatePayload{
				MinimumBattery: commandPayload,
			}, false)
		},
	}

	entities[MinumumBatteryTime] = HAEntityConfig{
		configTopic: "homeassistant/time/go_bms/minimum_battery_time_001/config",
		config: HATimeConfig{
			FriendlyName:   "Minimum Battery Time",
			Name:           "minimum_battery_time",
			UniqueDataID:   "minimum_battery_time_001",
			StateTopic:     mqttBMSStateTopic,
			ValueTemplate:  "{{ value_json.minimum_battery_time }}",
			CommandTopic:   "go_bms/minimum_battery_time/set",
			EntityCategory: HAEntityCategoryConfig,
			Device:         device,
			Icon:           "mdi:clock-outline",
		},
		subTopic: "go_bms/minimum_battery_time/set",
		subCallback: func(c mqtt.Client, m mqtt.Message) {
			commandPayload := string(m.Payload())
			b.Publish(StateUpdate{ID: string(MinumumBatteryTime), Value: commandPayload})
			s.updateState(HAStatePayload{
				MinimumBatteryTime: commandPayload,
			}, false)
		},
	}

	entities[MaximumDrainForMinimum] = HAEntityConfig{
		configTopic: "homeassistant/sensor/go_bms/maximum_drain_for_minimum_001/config",
		config: HASensorConfig{
			FriendlyName:      "Maximum Drain for Minimum",
			Name:              "maximum_drain_for_minimum",
			UniqueDataID:      "maximum_drain_for_minimum_001",
			StateTopic:        mqttBMSStateTopic,
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
			StateTopic:        mqttBMSStateTopic,
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
			StateTopic:             mqttBMSStateTopic,
			ValueTemplate:          "{{ value_json.energy_cost_forecast.current }}",
			JSONAttributesTopic:    mqttBMSStateTopic,
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
			StateTopic:             mqttBMSStateTopic,
			ValueTemplate:          "{{ value_json.energy_cost_forecast_tomorrow.current }}",
			JSONAttributesTopic:    mqttBMSStateTopic,
			JSONAttributesTemplate: "{% if value_json.energy_cost_forecast_tomorrow is defined %}{{ {'forecast': value_json.energy_cost_forecast_tomorrow.forecast} | tojson }}{% endif %}",
			UnitOfMeasurement:      "EUR/kWh",
			Icon:                   "mdi:cash",
			DeviceClass:            HADeviceClassMonetary,
			StateClass:             HAStateClassMeasurement,
			Device:                 device,
		},
	}

	return entities
}

func (s *service) initEntities(ctx context.Context, b *Broadcaster) {
	myDevice := HADevice{
		Identifiers:  []string{"go_bms_service"}, // Unique device ID
		Name:         "Go BMS",
		Model:        "v1.0 BMS",
		Manufacturer: "Go BMS",
		SwVersion:    "1.0.0",
		// SuggestedArea: "BMS",
	}
	entities := s.createEntities(myDevice, b)

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
		if !s.cfg.MQTTSetDefaults {
			return
		}
		time.Sleep(2 * time.Second) // Wait for entities to be registered in HA

		bmsEnabled := "false"
		if s.batteryManagement.BMSEnabled {
			bmsEnabled = "true"
		}
		bmsSimulationEnabled := "false"
		if s.batteryManagement.BMSSimulationEnabled {
			bmsSimulationEnabled = "true"
		}

		statePayload := HAStatePayload{
			BMSEnabled:             bmsEnabled,
			BMSSimulationEnabled:   bmsSimulationEnabled,
			BatteryDuration:        "0.0",
			MinimumBattery:         strconv.FormatFloat(s.batteryManagement.MinimumBatteryLevel, 'f', 2, 64),
			MinimumBatteryTime:     s.batteryManagement.MinimumBatteryTime,
			MaximumDrainForMinimum: "0.0",
			EnergyCostForecast: &ForcastData{
				Current: "0.0",
				Forecast: []*ForcastPoint{
					{DateTime: time.Now().UTC().Format(time.RFC3339), Value: "0.001"},
				},
			},
			EnergyCostForecastTomorrow: &ForcastData{
				Current: "0.0",
				Forecast: []*ForcastPoint{
					{DateTime: time.Now().Add(25 * time.Hour).UTC().Format(time.RFC3339), Value: "0.001"},
				},
			},
			EnergyCostCurrent: "0.0",
		}
		stateBytes, _ := json.Marshal(statePayload)

		s.mqttClient.Publish(mqttBMSStateTopic, 0, true, stateBytes)
		s.logger.InfoContext(ctx, "Published initial state data", slog.String("topic", mqttBMSStateTopic), slog.String("payload", string(stateBytes)))
	}()
}

func (s *service) updateState(state HAStatePayload, isSensor bool) {
	if s.batteryManagement.BMSEnabled || !isSensor {
		stateBytes, _ := json.Marshal(state)
		token := s.mqttClient.Publish(mqttBMSStateTopic, 0, false, stateBytes)
		token.Wait()
		if token.Error() != nil {
			s.logger.Error("Failed to publish state update", slog.Any("err", token.Error()))
		}
	}
}
