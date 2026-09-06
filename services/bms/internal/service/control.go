package service

import (
	"context"
	"log/slog"
	"time"
)

// Flow
// 1. number.solakon_one_modus_uberschreiben_dauer | force_duration -> time
// 2. select.solakon_one_modus_uberschreiben | force_mode -> 0 off, 1 discharge, 3 charge
// 3. number.solakon_one_modus_uberschreiben_leistung | force_power -> output power

func (s *service) readCurrentControls(ctx context.Context) {
	chForceDuration := s.broad.Subscribe(sensorSolakonForceDurationEntityID, 1)
	chForceMode := s.broad.Subscribe(sensorSolakonForceModeEntityID, 1)
	chForcePower := s.broad.Subscribe(sensorSolakonForcePowerEntityID, 1)
	chSimForceMode := s.broad.Subscribe(string(SimForceMode), 1)

	for {
		select {
		case <-ctx.Done():
			return
		case forceDuration := <-chForceDuration:
			// 42.0
			s.logger.Info("CurrentControls: Force duration update", "value", forceDuration.Value)
		case forceMode := <-chForceMode:
			// off = 0, discharge = 1, charge = 3
			s.logger.Info("CurrentControls: Force mode update", "value", forceMode.Value)
		case forcePower := <-chForcePower:
			// 230
			s.logger.Info("CurrentControls: Force power update", "value", forcePower.Value)
		case simForceMode := <-chSimForceMode:
			// off = 0, discharge = 1, charge = 3
			s.logger.Info("CurrentControls: Sim Force Mode update", "value", simForceMode.Value)
		}
	}
}

func (s *service) addControls(ctx context.Context) {
	go s.handleControlMode(ctx)
	go s.handleControlTime(ctx)
	go s.handleAutoMode(ctx)
}

func (s *service) setControlTime(ctx context.Context) {
	controlDuration := s.batteryManagement.GetControlDuration()
	bmsActive := s.batteryManagement.GetBMSEnabled()

	if !bmsActive {
		return
	}

	now := time.Now()
	controlTime := now.Add(controlDuration)
	s.batteryManagement.ControlTime = &controlTime
	controlTimeStr := formatTimestamp(&controlTime)
	s.broad.Publish(StateUpdate{ID: string(ControlTime), Value: controlTimeStr})
}

func (s *service) handleControlTime(ctx context.Context) {
	chControlTime := s.broad.Subscribe(string(ControlTime), 1)
	var controlTime *time.Time

	// use non cancalable context to ensure that the reset happens even if the main context is canceled
	defer s.resetMode(context.Background())
	for {
		select {
		case <-ctx.Done():
			return
		case v := <-chControlTime:
			controlTimeStr := v.Value
			if ct, ok := parseTimestamp(controlTimeStr); ok {
				controlTime = ct
			} else {
				continue
			}

			// wait until the control time is reached
			now := time.Now()
			if controlTime.After(now) {
				duration := controlTime.Sub(now)
				s.logger.Info("Waiting until control time is reached", slog.String("control_time", controlTimeStr), slog.String("duration", duration.String()))

				sleepWithContext(ctx, duration)

				s.logger.Info("Control time reached", slog.String("control_time", controlTimeStr))
				s.resetMode(ctx)
			}
		}
	}
}

func (s *service) handleControlMode(ctx context.Context) {
	chControlMode := s.broad.Subscribe(string(ControlMode), 1)
	chBMSActive := s.broad.Subscribe(string(BMS), 1)
	bmsActive := s.batteryManagement.GetBMSEnabled()
	controlMode := s.batteryManagement.ControlMode
	for {
		select {
		case <-ctx.Done():
			return
		case v := <-chControlMode:
			controlMode = ControlModeOpt(v.Value)
		case v := <-chBMSActive:
			bmsActive = v.Value == "true"
		}

		if !bmsActive {
			continue
		}

		switch controlMode {
		case ControlModeOptOff:
			s.logger.Info("Control mode set to off, resetting control time and duration")
			s.resetMode(ctx)
			continue
		case ControlModeOptDischarge:
			s.logger.Info("Control mode set to discharge, setting control time and duration")
			s.setControlTime(ctx)
			s.dischargeBattery(ctx, s.batteryManagement.ControlPower, s.batteryManagement.ControlDuration)
		case ControlModeOptCharge:
			s.logger.Info("Control mode set to charge, setting control time and duration")
			s.setControlTime(ctx)
			s.chargeBattery(ctx, s.batteryManagement.ControlPower, s.batteryManagement.ControlDuration)
		}
	}
}

func (s *service) handleAutoMode(ctx context.Context) {
	chAutoMode := s.broad.Subscribe(string(AutoMode), 1)
	chControlMode := s.broad.Subscribe(string(ControlMode), 1)
	chBMSActive := s.broad.Subscribe(string(BMS), 1)
	autoMode := s.batteryManagement.AutoMode
	controlMode := s.batteryManagement.ControlMode
	bmsActive := s.batteryManagement.GetBMSEnabled()

	cancelCtx, cancel := context.WithCancel(ctx)

	for {
		select {
		case <-ctx.Done():
			cancel()
			// reset
			s.resetMode(ctx)
			s.broad.Publish(StateUpdate{ID: string(AutoMode), Value: string(AutoModeOptOff)})
			return
		case v := <-chAutoMode:
			autoMode = AutoModeOpt(v.Value)
		case v := <-chControlMode:
			controlMode = ControlModeOpt(v.Value)
		case v := <-chBMSActive:
			bmsActive = v.Value == "true"
		}

		s.logger.Info("Auto mode", slog.String("auto_mode", string(autoMode)), slog.String("control_mode", string(controlMode)), slog.Bool("bms_active", bmsActive))
		if !bmsActive || controlMode != ControlModeOptAuto {
			cancel()
			// not active, or not in auto mode, skip
			continue
		}

		switch autoMode {
		case AutoModeOptOff:
			s.logger.Info("Auto mode set to off, resetting control time and duration")
			s.resetMode(ctx)
		case AutoModeOptChargeWhenNeeded:
			// get best window for charging
			s.autoModeChargeWhenNeeded(cancelCtx)
		}
	}
}

func (s *service) autoModeChargeWhenNeeded(ctx context.Context) {
	chLevel := s.broad.Subscribe(sensorSolakonBatteryLevelEntityID, 1)
	chMaxChargeLevel := s.broad.Subscribe(string(AutoMaxChargeLevel), 1)
	level := s.solakon.Initials.BatteryLevel
	maxChargeLevel := s.batteryManagement.AutoMaxChargeLevel
	batteryCapacity := s.solakon.BatteryCapacity

	dailyEnergy := s.solakon.Initials.DailyTotalEnergy
	productionToday := s.solarProduction.EnergyProductionToday

	chargingPower := s.batteryManagement.ControlPower
	isCharging := false

	chTimer := time.After(time.Millisecond)

	doCharging := func() {
		limit := min(maxChargeLevel, 99) // cap for safety
		// current energy in battery
		batteryEnergy := batteryCapacity * (level / 100.0)
		chargeEnergy := batteryCapacity * (limit / 100.0)
		amountToCharge := chargeEnergy - batteryEnergy
		if level >= limit {
			s.logger.Info("Battery is fully charged, resetting control time and duration",
				slog.Float64("level", level),
				slog.Float64("limit", limit),
			)
			s.resetMode(ctx)
			isCharging = false
			return
		}
		s.logger.Info("AutoMode ChargeWhenNeeded",
			slog.Float64("current_battery_energy", batteryEnergy),
			slog.Float64("charge_energy", chargeEnergy),
			slog.Float64("current_level", level),
			slog.Float64("limit", limit),
			slog.Float64("amount",
				amountToCharge),
			slog.Float64("daily_energy", dailyEnergy),
			slog.Float64("production_today", productionToday),
			slog.Float64("pv_production", productionToday-dailyEnergy),
		)

		now := time.Now()
		// if after 15:00 do charge
		if now.Hour() >= 15 {
			s.logger.Info("Enforce charging after 15:00, ignoring PV production", slog.Int("hour", now.Hour()))
			// expected amout to charge until sunset
			// amountToCharge -= 0.5
		} else if amountToCharge <= productionToday-dailyEnergy {
			// TODO:
			s.logger.Info("Battery is expected to be charged by PV production, skipping charging")
			return
		} else {
			amountToCharge -= productionToday - dailyEnergy
		}
		spots, err := s.priceForecastCache.GetBestChargingSpots(amountToCharge, chargingPower, now)
		if err != nil || len(spots) == 0 {
			s.logger.Error("Failed to get best charging spots", slog.Any("err", err))
			return
		}
		s.logger.Info("Best charging spots", slog.Any("spots", spots))

		// check is spot is currently active, if so, start charging
		if !isCharging && now.After(spots[0][0]) && now.Before(spots[0][1]) {
			s.logger.Info("Charging spot is currently active, starting charge", slog.Any("spot", spots[0]))
			duration := spots[0][1].Sub(now)
			s.chargeBattery(context.Background(), chargingPower, duration)
			s.batteryManagement.SetControlDuration(duration)
			s.setControlTime(context.Background())
			isCharging = true
		}
		// wait until the next spot is active
		if !isCharging && now.Before(spots[0][0]) {
			duration := spots[0][0].Sub(now)
			s.logger.Info("Waiting until next charging spot is active", slog.Any("spot", spots[0]), slog.String("duration", duration.String()))
			chTimer = time.After(duration)
		}
	}
	doCharging()

	for {
		select {
		case <-ctx.Done():
			return
		case v := <-chLevel:
			level = parseFloat(v.Value)
		case v := <-chMaxChargeLevel:
			maxChargeLevel = parseFloat(v.Value)
		case <-chTimer:
		}
		doCharging()
	}
}

func (s *service) resetMode(ctx context.Context) {
	if s.batteryManagement.ControlMode != ControlModeOptAuto {
		s.broad.Publish(StateUpdate{ID: string(ControlMode), Value: string(ControlModeOptOff)})
	}
	s.broad.Publish(StateUpdate{ID: string(ControlTime), Value: ""})
	s.broad.Publish(StateUpdate{ID: string(SimForceDuration), Value: formatFloatDuration(0)})
	s.broad.Publish(StateUpdate{ID: string(SimForceMode), Value: string(ControlModeOptOff)})
	s.broad.Publish(StateUpdate{ID: string(SimForcePower), Value: formatFloat(0)})

	if s.batteryManagement.GetBMSSimulationEnabled() {
		s.logger.Info("Skipping reset for active control, BMS simulation is enabled")
		return
	}
	// reset the control time and duration
	_, err := s.haClient.CallServiceForEntity(ctx, "number", "set_value", sensorSolakonForceDurationEntityID, map[string]interface{}{
		"value": 0,
	})
	if err != nil {
		s.logger.Error("Failed to set force duration", slog.Any("err", err))
		return
	}
}

func (s *service) chargeBattery(ctx context.Context, power float64, duration time.Duration) {
	s.broad.Publish(StateUpdate{ID: string(SimForceDuration), Value: formatFloatDuration(duration)})
	s.broad.Publish(StateUpdate{ID: string(SimForceMode), Value: string(ControlModeOptCharge)})
	s.broad.Publish(StateUpdate{ID: string(SimForcePower), Value: formatFloat(power)})

	if s.batteryManagement.GetBMSSimulationEnabled() {
		s.logger.Info("Skipping charge battery, BMS simulation is enabled")
		return
	}
	// 1. number.solakon_one_modus_uberschreiben_dauer | force_duration -> time
	_, err := s.haClient.CallServiceForEntity(ctx, "number", "set_value", sensorSolakonForceDurationEntityID, map[string]interface{}{
		"value": formatFloatDuration(duration),
	})
	if err != nil {
		s.logger.Error("Charge: Failed to set force duration", slog.Any("err", err), slog.Any("duration", duration))
		return
	}
	// 3. number.solakon_one_modus_uberschreiben_leistung | force_power -> output power
	_, err = s.haClient.CallServiceForEntity(ctx, "number", "set_value", sensorSolakonForcePowerEntityID, map[string]interface{}{
		"value": formatFloat(power),
	})
	if err != nil {
		s.logger.Error("Charge: Failed to set force power", slog.Any("err", err), slog.Any("power", power))
		return
	}
	// 2. select.solakon_one_modus_uberschreiben | force_mode -> 0 off, 1 discharge, 3 charge
	_, err = s.haClient.CallServiceForEntity(ctx, "select", "select_option", sensorSolakonForceModeEntityID, map[string]interface{}{
		"option": string(SolakonForceModeCharge),
	})
	if err != nil {
		s.logger.Error("Charge: Failed to set force mode", slog.Any("err", err))
		return
	}
}

func (s *service) dischargeBattery(ctx context.Context, power float64, duration time.Duration) {
	s.broad.Publish(StateUpdate{ID: string(SimForceDuration), Value: formatFloatDuration(duration)})
	s.broad.Publish(StateUpdate{ID: string(SimForceMode), Value: string(ControlModeOptDischarge)})
	s.broad.Publish(StateUpdate{ID: string(SimForcePower), Value: formatFloat(power)})

	if s.batteryManagement.GetBMSSimulationEnabled() {
		s.logger.Info("Skipping discharge battery, BMS simulation is enabled")
		return
	}
	// number.solakon_one_modus_uberschreiben_dauer | force_duration -> time
	_, err := s.haClient.CallServiceForEntity(ctx, "number", "set_value", sensorSolakonForceDurationEntityID, map[string]interface{}{
		"value": formatFloatDuration(duration),
	})
	if err != nil {
		s.logger.Error("Discharge: Failed to set force duration", slog.Any("err", err), slog.Any("duration", duration))
		return
	}
	// number.solakon_one_modus_uberschreiben_leistung | force_power -> output power
	_, err = s.haClient.CallServiceForEntity(ctx, "number", "set_value", sensorSolakonForcePowerEntityID, map[string]interface{}{
		"value": formatFloat(power),
	})
	if err != nil {
		s.logger.Error("Discharge: Failed to set force power", slog.Any("err", err), slog.Any("power", power))
		return
	}
	// select.solakon_one_modus_uberschreiben | force_mode -> 0 off, 1 discharge, 3 charge
	_, err = s.haClient.CallServiceForEntity(ctx, "select", "select_option", sensorSolakonForceModeEntityID, map[string]interface{}{
		"option": string(SolakonForceModeDischarge),
	})
	if err != nil {
		s.logger.Error("Discharge: Failed to set force mode", slog.Any("err", err))
		return
	}
}
