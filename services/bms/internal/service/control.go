package service

import "context"

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
