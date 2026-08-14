package service

import (
	"context"
	"log/slog"
)

func (s *service) startSubscriptions(ctx context.Context, b *Broadcaster) {
	go s.subscribeBMS(ctx, b)
	go s.subscribeBMSSimulation(ctx, b)
	go s.subscribeSolakonBatteryPower(ctx, b)

	// TODO: Subscribe once and filter state_changed events for selected entities.
	// sub, err := ws.SubscribeStateChangedMany(context.Background(), watchedEntities...)
}

func (s *service) subscribeBMS(ctx context.Context, b *Broadcaster) {
	sub, err := s.haWs.SubscribeStateChanged(ctx, bmsEnabledEntityID)
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
			b.Publish(StateUpdate{ID: data.EntityID, Value: data.NewState.State})
			s.logger.Info("HA event received", slog.String("entity_id", data.EntityID), slog.String("old_state", oldState), slog.String("new_state", data.NewState.State))
		}
	}
}

func (s *service) subscribeBMSSimulation(ctx context.Context, b *Broadcaster) {
	sub, err := s.haWs.SubscribeStateChanged(ctx, bmsSimulationEnabledEntityID)
	if err != nil {
		s.logger.Error("Failed to subscribe to HA events", slog.Any("err", err))
		return
	}
	defer sub.Unsubscribe(context.Background())
	s.logger.Info("Started sub")
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
			s.logger.Info("HA event received")
			// Decode the event payload to strongly-typed state_changed data.
			data, ok, err := ev.StateChanged()
			if err != nil || !ok || data.NewState == nil {
				continue
			}
			oldState := ""
			if data.OldState != nil {
				oldState = data.OldState.State
			}
			b.Publish(StateUpdate{ID: data.EntityID, Value: data.NewState.State})
			s.logger.Info("HA event received", slog.String("entity_id", data.EntityID), slog.String("old_state", oldState), slog.String("new_state", data.NewState.State))
		}
	}
}

func (s *service) subscribeSolakonBatteryPower(ctx context.Context, b *Broadcaster) {
	sub, err := s.haWs.SubscribeStateChanged(ctx, sensorSolakonBatteryPowerEntityID)
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
			b.Publish(StateUpdate{ID: data.EntityID, Value: data.NewState.State})
			s.logger.Info("HA event received", slog.String("entity_id", data.EntityID), slog.String("old_state", oldState), slog.String("new_state", data.NewState.State))
		}
	}
}
