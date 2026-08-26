package service

import (
	"context"
	"log/slog"
	"time"
)

func (s *service) batteryDrainByBatteryPower(ctx context.Context, b *Broadcaster) {

	chDrain := b.Subscribe(sensorSolakonBatteryPowerEntityID, 1)
	chLevel := b.Subscribe(sensorSolakonBatteryLevelEntityID, 1)
	chBMS := b.Subscribe(string(BMS), 1)

	drain := s.solakon.Initials.BatteryPower
	level := s.solakon.Initials.BatteryLevel

	calculateAndPublish := func() {
		if drain <= 0 || level <= 0 {
			s.updateState(HAStatePayload{
				BatteryDuration: "0.0",
			}, true)
			return
		}
		// calculates battery duration in hours
		duration := calculateBatteryDuration(s.solakon.BatteryCapacity, level, s.solakon.BatteryEfficiency, drain)

		s.logger.Debug("Battery duration calculated", slog.Float64("duration", duration))

		s.updateState(HAStatePayload{
			BatteryDuration: formatFloat(duration),
		}, true)
	}

	calculateAndPublish()

	for {
		select {
		case d := <-chDrain:
			drain = parseFloat(d.Value)
		case l := <-chLevel:
			level = parseFloat(l.Value)
		case <-ctx.Done():
			return
		case <-chBMS: // do run
		}

		calculateAndPublish()
	}

}

func (s *service) batteryMaxDrainUntilLevel(ctx context.Context, b *Broadcaster) {
	chLevel := b.Subscribe(sensorSolakonBatteryLevelEntityID, 1)
	chDrain := b.Subscribe(sensorSolakonBatteryPowerEntityID, 1)
	chMinLevel := b.Subscribe(string(MinumumBattery), 1)
	chMinTime := b.Subscribe(string(MinumumBatteryTime), 1)
	chBMS := b.Subscribe(string(BMS), 1)

	drain := 0.0
	level := s.solakon.Initials.BatteryLevel
	minLevel := s.batteryManagement.MinimumBatteryLevel
	// fallback to 00:00:00 if not set
	targetTime := time.Now().Add(24 * time.Hour).Truncate(24 * time.Hour)
	targetTimeParsed, ok := targetTimeFromString(s.batteryManagement.MinimumBatteryTime)
	if ok {
		targetTime = targetTimeParsed
	}

	calculateAndPublish := func() {
		if minLevel > level || drain <= 10 {
			// if level is lower as min
			// if drain is lower as 10W
			s.updateState(HAStatePayload{
				MaximumDrainForMinimum: "0.0",
			}, true)
			return
		}
		usableWattHours := s.solakon.BatteryCapacity * 1000 * (level - minLevel) / 100 * s.solakon.BatteryEfficiency

		// targetTime = next day at 10:00 AM
		hoursUntilTarget := targetTime.Sub(time.Now()).Hours()
		maxDrain := usableWattHours / hoursUntilTarget

		s.logger.Debug("Max drain calculated", slog.Float64("max_drain", maxDrain), slog.Time("at", targetTime))

		// cap max drain at ACMaxAllowedPower (800W)
		if maxDrain > s.solakon.ACMaxAllowedPower {
			maxDrain = s.solakon.ACMaxAllowedPower
		}

		s.updateState(HAStatePayload{
			MaximumDrainForMinimum: formatFloat(maxDrain),
		}, true)
	}

	calculateAndPublish()

	for {
		select {
		case l := <-chLevel:
			level = parseFloat(l.Value)
		case m := <-chMinLevel:
			minLevel = parseFloat(m.Value)
		case d := <-chDrain:
			drain = parseFloat(d.Value)
		case mt := <-chMinTime:
			// parse time in format "HH:MM:SS"
			targetTimeParsed, ok := targetTimeFromString(mt.Value)
			if ok {
				targetTime = targetTimeParsed
			}
		case <-ctx.Done():
			return
		case <-chBMS: // do run
		}
		calculateAndPublish()
	}
}

func calculateBatteryDuration(batteryCapacity, batteryLevel, batteryEfficiency, drain float64) float64 {

	batteryAvailable := batteryLevel / 100 * batteryCapacity
	batteryDuration := (batteryAvailable * 1000 * batteryEfficiency) / drain

	return batteryDuration
}
