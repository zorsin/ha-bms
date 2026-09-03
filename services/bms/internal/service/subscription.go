package service

import (
	"context"
	"log/slog"
	"sync"
)

const (
	// solakon one
	sensorSolakonBatteryPowerEntityID    = "sensor.solakon_one_batterie_leistung"
	sensorSolakonBatteryCapacityEntityID = "sensor.solakon_one_batteriekapazitat"
	sensorSolakonBatteryLevelEntityID    = "sensor.solakon_one_batterie_ladestand"
	sensorSolakonForcePowerEntityID      = "number.solakon_one_modus_uberschreiben_leistung"
	sensorSolakonForceDurationEntityID   = "number.solakon_one_modus_uberschreiben_dauer"
	sensorSolakonForceModeEntityID       = "select.solakon_one_modus_uberschreiben"
	sensorSolakonDailyEnergyEntityID     = "sensor.solakon_one_tagliche_energie"

	// solar energy forecast
	sensorSolarEnergyProductionTodayEntityID          = "sensor.energy_production_today"
	sensorSolarEnergyProductionTodayRemainingEntityID = "sensor.energy_production_today_remaining"
)

func (s *service) startSubscriptions(ctx context.Context) {
	watchedEntities := []string{
		sensorSolakonBatteryPowerEntityID,
		sensorSolakonBatteryLevelEntityID,
		sensorSolakonForcePowerEntityID,
		sensorSolakonForceDurationEntityID,
		sensorSolakonForceModeEntityID,
		sensorSolakonDailyEnergyEntityID,
		sensorSolarEnergyProductionTodayEntityID,
		sensorSolarEnergyProductionTodayRemainingEntityID,
	}
	// Subscribe once and filter state_changed events for selected entities.
	sub, err := s.haWs.SubscribeStateChangedMany(ctx, watchedEntities...)
	if err != nil {
		s.logger.Error("Failed to subscribe to HA events", slog.Any("err", err))
		return
	}
	defer sub.Unsubscribe(context.Background())
	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Done", slog.Any("err", ctx.Err()))
			return
		case err := <-sub.Errors():
			if err != nil {
				s.logger.Error("subscription error", slog.Any("err", err))
			}
		case ev, ok := <-sub.Events():
			if !ok {
				s.logger.Info("subscription closed")
				return
			}
			// Decode the event payload to strongly-typed state_changed data.
			data, ok, err := ev.StateChanged()
			if err != nil || !ok || data.NewState == nil {
				continue
			}
			oldState := ""
			if data.OldState != nil {
				oldState = data.OldState.State
			}
			s.broad.Publish(StateUpdate{ID: data.EntityID, Value: data.NewState.State})
			s.logger.Debug("HA event received", slog.String("entity_id", data.EntityID), slog.String("old_state", oldState), slog.String("new_state", data.NewState.State))
		}
	}
}

func (s *service) getStaticStates(ctx context.Context) bool {
	solakon := &SolakonOne{
		BatteryEfficiency: 0.85,
		ACMaxAllowedPower: 800,
	}
	batteryManagement := &BatteryManagement{
		mux:                  &sync.RWMutex{},
		BMSEnabled:           false,
		BMSSimulationEnabled: true,
		MinimumBatteryLevel:  20,
		MinimumBatteryTime:   "10:00:00",
		ControlDuration:      0,
		ControlTime:          nil,
		ControlPower:         0,
		ControlMode:          ControlModeOptOff,
		AutoMode:             AutoModeOptOff,
	}

	totalOk := true
	state, err := s.haClient.GetStateForEntity(ctx, sensorSolakonBatteryCapacityEntityID)
	if err != nil {
		totalOk = false
	}
	solakon.BatteryCapacity = parseFloat(state.State)

	state, err = s.haClient.GetStateForEntity(ctx, sensorSolakonBatteryLevelEntityID)
	if err != nil {
		totalOk = false
	}
	solakon.Initials.BatteryLevel = parseFloat(state.State)

	state, err = s.haClient.GetStateForEntity(ctx, sensorSolakonBatteryPowerEntityID)
	if err != nil {
		totalOk = false
	}
	solakon.Initials.BatteryPower = parseFloat(state.State)

	state, err = s.haClient.GetStateForEntity(ctx, sensorSolakonDailyEnergyEntityID)
	if err != nil {
		totalOk = false
	}
	solakon.Initials.DailyTotalEnergy = parseFloat(state.State)
	//
	// Solar production forecast
	//
	state, err = s.haClient.GetStateForEntity(ctx, sensorSolarEnergyProductionTodayEntityID)
	if err != nil {
		totalOk = false
	}
	s.solarProduction.EnergyProductionToday = parseFloat(state.State)

	state, err = s.haClient.GetStateForEntity(ctx, sensorSolarEnergyProductionTodayRemainingEntityID)
	if err != nil {
		totalOk = false
	}
	s.solarProduction.EnergyProductionTodayRemaining = parseFloat(state.State)

	//
	// BMS
	//
	state, err = s.haClient.GetStateForEntity(ctx, string(MinumumBattery))
	if err == nil {
		batteryManagement.MinimumBatteryLevel = parseFloat(state.State)
	}
	state, err = s.haClient.GetStateForEntity(ctx, string(MinumumBatteryTime))
	if err == nil {
		batteryManagement.MinimumBatteryTime = state.State
	}
	state, err = s.haClient.GetStateForEntity(ctx, string(BMS))
	if err == nil {
		batteryManagement.SetBMSEnabled(state.State == "on" || state.State == "true")
	}
	state, err = s.haClient.GetStateForEntity(ctx, string(BMSSimulation))
	if err == nil {
		batteryManagement.SetBMSSimulationEnabled(state.State == "on" || state.State == "true")
	}
	state, err = s.haClient.GetStateForEntity(ctx, string(ControlDuration))
	if err == nil {
		batteryManagement.ControlDuration = parseFloatDuration(state.State)
	}
	state, err = s.haClient.GetStateForEntity(ctx, string(ControlTime))
	if err == nil {
		if t, ok := parseTimestamp(state.State); ok {
			batteryManagement.ControlTime = t
		}
	}
	state, err = s.haClient.GetStateForEntity(ctx, string(ControlPower))
	if err == nil {
		batteryManagement.ControlPower = parseFloat(state.State)
	}
	state, err = s.haClient.GetStateForEntity(ctx, string(AutoMaxChargeLevel))
	if err == nil {
		batteryManagement.AutoMaxChargeLevel = parseFloat(state.State)
	}
	// state, err = s.haClient.GetStateForEntity(ctx, string(ControlMode))
	// if err == nil {
	// 	batteryManagement.ControlMode = ControlModeOpt(state.State)
	// }
	// state, err = s.haClient.GetStateForEntity(ctx, string(AutoMode))
	// if err == nil {
	// 	batteryManagement.AutoMode = AutoModeOpt(state.State)
	// }

	s.solakon = solakon
	s.batteryManagement = batteryManagement
	s.logger.Info("Initial states", slog.Any("solakon", solakon), slog.Any("batteryManagement", batteryManagement))
	return totalOk
}
