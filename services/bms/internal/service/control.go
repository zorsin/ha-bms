package service

import (
	"context"
	"log/slog"
	"time"

	go_ha_client "github.com/mkelcik/go-ha-client/v2"
)

// Flow
// 1. number.solakon_one_modus_uberschreiben_dauer | force_duration -> time
// 2. select.solakon_one_modus_uberschreiben | force_mode -> 0 off, 1 discharge, 3 charge
// 3. number.solakon_one_modus_uberschreiben_leistung | force_power -> output power

func (s *service) readCurrentControls(ctx context.Context, b *Broadcaster) {
	chForceDuration := b.Subscribe(sensorSolakonForceDurationEntityID, 1)
	chForceMode := b.Subscribe(sensorSolakonForceModeEntityID, 1)
	chForcePower := b.Subscribe(sensorSolakonForcePowerEntityID, 1)
	chSimForceMode := b.Subscribe(string(SimForceMode), 1)

	for {
		select {
		case <-ctx.Done():
			return
		case forceDuration := <-chForceDuration:
			// 42.0
			s.logger.Info("Force duration update", "value", forceDuration.Value)
		case forceMode := <-chForceMode:
			// off = 0, discharge = 1, charge = 3
			s.logger.Info("Force mode update", "value", forceMode.Value)
		case forcePower := <-chForcePower:
			// 230
			s.logger.Info("Force power update", "value", forcePower.Value)
		case simForceMode := <-chSimForceMode:
			// off = 0, discharge = 1, charge = 3
			s.logger.Info("Sim Force Mode update", "value", simForceMode.Value)
		}
	}
}

func (s *service) addControls(ctx context.Context, b *Broadcaster) {
	go s.setSimForceDuration(ctx, b)
	go s.handleControlMode(ctx, b)
	go s.handleControlTime(ctx, b)
}

func (s *service) setSimForceDuration(ctx context.Context, b *Broadcaster) {
	chControlDuration := b.Subscribe(string(ControlDuration), 1)
	controlDuration := "0"
	for {
		select {
		case <-ctx.Done():
			return
		case v := <-chControlDuration:
			controlDuration = v.Value
		}

		b.Publish(StateUpdate{ID: string(SimForceDuration), Value: controlDuration})
		s.updateState(HAStatePayload{
			SimForceDuration: controlDuration,
		}, true)
	}
}

func (s *service) setControlTime(ctx context.Context, b *Broadcaster) {
	controlDuration := s.batteryManagement.ControlDuration
	bmsActive := s.batteryManagement.GetBMSEnabled()

	if !bmsActive {
		return
	}

	now := time.Now()
	controlTime := now.Add(controlDuration)
	s.batteryManagement.ControlTime = &controlTime
	controlTimeStr := formatTimestamp(&controlTime)
	b.Publish(StateUpdate{ID: string(ControlTime), Value: controlTimeStr})
	s.updateState(HAStatePayload{
		ControlTime: controlTimeStr,
	}, true)
}

func (s *service) handleControlTime(ctx context.Context, b *Broadcaster) {
	chControlTime := b.Subscribe(string(ControlTime), 1)
	var controlTime *time.Time
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
				time.Sleep(duration)

				s.logger.Info("Control time reached", slog.String("control_time", controlTimeStr))
				s.resetMode(ctx)
			}
		}
	}
}

func (s *service) handleControlMode(ctx context.Context, b *Broadcaster) {
	chControlMode := b.Subscribe(string(ControlMode), 1)
	chBMSActive := b.Subscribe(string(BMS), 1)
	bmsActive := s.batteryManagement.GetBMSEnabled()

	for {
		select {
		case <-ctx.Done():
			return
		case v := <-chControlMode:
			controlMode := ControlModeOpt(v.Value)
			s.batteryManagement.ControlMode = controlMode
		case v := <-chBMSActive:
			bmsActive = v.Value == "true"
		}

		if !bmsActive {
			continue
		}

		switch s.batteryManagement.ControlMode {
		case ControlModeOptOff:
			s.logger.Info("Control mode set to off, resetting control time and duration")
			continue
		case ControlModeOptDischarge:
			s.logger.Info("Control mode set to discharge, setting control time and duration")
			//
		case ControlModeOptCharge:
			s.logger.Info("Control mode set to charge, setting control time and duration")
			// TODO: battery power
			s.setControlTime(ctx, b)
			s.chargeBattery(ctx, 800, s.batteryManagement.ControlDuration)
		}
	}
}

func (s *service) resetMode(ctx context.Context) {
	s.updateState(HAStatePayload{
		ControlMode:      string(ControlModeOptOff),
		ControlTime:      "",
		SimForceDuration: formatFloatDuration(0),
		SimForceMode:     "off",
		SimForcePower:    formatFloat(0),
	}, false)

	if s.batteryManagement.GetBMSSimulationEnabled() {
		s.logger.Info("Skipping reset for active control, BMS simulation is enabled")
		return
	}
	// reset the control time and duration
	_, err := s.haClient.CreateState(ctx, sensorSolakonForceDurationEntityID, go_ha_client.State{
		State: formatFloatDuration(0),
	})
	if err != nil {
		s.logger.Error("Failed to set force duration", slog.Any("err", err))
		return
	}
	/*
		// 2. select.solakon_one_modus_uberschreiben | force_mode -> 0 off, 1 discharge, 3 charge
		_, err = s.haClient.CreateState(ctx, sensorSolakonForceModeEntityID, go_ha_client.State{
			State: "3", // charge
		})
		if err != nil {
			s.logger.Error("Failed to set force mode", slog.Any("err", err))
			return
		}
		// 3. number.solakon_one_modus_uberschreiben_leistung | force_power -> output power
		_, err = s.haClient.CreateState(ctx, sensorSolakonForcePowerEntityID, go_ha_client.State{
			State: formatFloat(0),
		})
		if err != nil {
			s.logger.Error("Failed to set force power", slog.Any("err", err))
			return
		}
	*/
}

func (s *service) chargeBattery(ctx context.Context, power float64, duration time.Duration) {
	s.updateState(HAStatePayload{
		SimForceDuration: formatFloatDuration(duration),
		SimForceMode:     "charge",
		SimForcePower:    formatFloat(power),
	}, true)

	if s.batteryManagement.GetBMSSimulationEnabled() {
		s.logger.Info("Skipping charge battery, BMS simulation is enabled")
		return
	}
	// 1. number.solakon_one_modus_uberschreiben_dauer | force_duration -> time
	_, err := s.haClient.CreateState(ctx, sensorSolakonForceDurationEntityID, go_ha_client.State{
		State: formatFloatDuration(duration),
	})
	if err != nil {
		s.logger.Error("Failed to set force duration", slog.Any("err", err))
		return
	}
	// 2. select.solakon_one_modus_uberschreiben | force_mode -> 0 off, 1 discharge, 3 charge
	_, err = s.haClient.CreateState(ctx, sensorSolakonForceModeEntityID, go_ha_client.State{
		State: "3", // charge
	})
	if err != nil {
		s.logger.Error("Failed to set force mode", slog.Any("err", err))
		return
	}
	// 3. number.solakon_one_modus_uberschreiben_leistung | force_power -> output power
	_, err = s.haClient.CreateState(ctx, sensorSolakonForcePowerEntityID, go_ha_client.State{
		State: formatFloat(power),
	})
	if err != nil {
		s.logger.Error("Failed to set force power", slog.Any("err", err))
		return
	}
}
