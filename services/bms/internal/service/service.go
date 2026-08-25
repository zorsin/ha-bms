package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	ha "github.com/mkelcik/go-ha-client/v2"
	"github.com/zorsin/ha-bms/services/bms/internal/config"
)

type service struct {
	cfg               *config.Config
	logger            *slog.Logger
	haClient          *ha.Client
	haWs              *ha.WSClient
	mqttClient        mqtt.Client
	solakon           *SolakonOne
	batteryManagement *BatteryManagement
}

const (
	initializationWaitTime = 5 * time.Second
)

func New(cfg *config.Config, logger *slog.Logger) *service {

	client, err := ha.NewClient(cfg.HAURL, ha.WithToken(cfg.HAToken))
	if err != nil {
		logger.Error("Failed to create HA client", slog.Any("err", err))
		return nil
	}

	haWs := client.WS()
	srv := &service{
		cfg:      cfg,
		logger:   logger,
		haClient: client,
		haWs:     haWs,
	}

	return srv
}

func (s *service) Start(ctx context.Context) {
	s.logger.Info("Starting service")
	err := s.haWs.Connect(ctx)
	if err != nil {
		s.logger.Error("Failed to connect to HA WebSocket", slog.Any("err", err))
		return
	}
	err = s.initMQTTClient(ctx)
	if err != nil {
		s.logger.Error("Failed to initialize MQTT client", slog.Any("err", err))
		return
	}
	if s.cfg.BMSActive {
		s.logger.Info("BMS is active, initializing service")
	} else {
		s.logger.Info("BMS is not active, skipping service initialization")
		return
	}
	go s.initService(ctx)
}

func (s *service) Shutdown() {
	s.logger.Info("Shutting down service")
	s.haWs.Close()
}

// return patter and handler for http server
func (s *service) GetHTTPHandlers() map[string]http.Handler {
	return map[string]http.Handler{
		"/service/status": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			type statusResponse struct {
				Status   string `json:"status"`
				HAStatus string `json:"ha_status"`
			}
			status := statusResponse{
				Status:   "ok",
				HAStatus: "unknown",
			}
			err := s.haClient.Ping(context.Background())
			if err != nil {
				status.HAStatus = "error"
			} else {
				status.HAStatus = "ok"
			}

			data, err := json.Marshal(status)

			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			w.Write(data)
		}),
		"/service/ha/config": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			haCfg := s.PrintHAConfig(context.Background())

			data, _ := json.Marshal(haCfg)

			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			w.Write(data)
		}),
		"/service/ha/sensors": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// get url query param "entity_id"
			entityID := r.URL.Query().Get("id")

			// state, _ := s.haClient
			// get entity config for entityID
			// s.haClient.Get

			data, _ := json.Marshal(entityID)

			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			w.Write(data)
		}),
		"/service/ha/state": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// get url query param "entity_id"
			entityID := r.URL.Query().Get("id")

			state, _ := s.haClient.GetStateForEntity(r.Context(), entityID)

			data, _ := json.Marshal(state)

			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			w.Write(data)
		}),
	}
}

// print ha config
func (s *service) PrintHAConfig(ctx context.Context) *ha.Config {
	haCfg, err := s.haClient.GetConfig(ctx)
	if err != nil {
		return nil
	}

	return &haCfg
}

func (s *service) initService(ctx context.Context) {
	s.logger.Info("Initializing service")
	b := NewBroadcaster()
	// get statics states
	ok := s.getStaticStates(ctx)
	if !ok {
		s.logger.Warn("Failed to get static states")
		return
	}
	s.initEntities(ctx, b)
	// wait for entities to be registered in HA
	time.Sleep(initializationWaitTime)
	// get initial states
	// subscribe to HA events
	go s.startSubscriptions(ctx, b)

	go s.batteryDrainByBatteryPower(ctx, b)
	go s.batteryMaxDrainUntilLevel(ctx, b)
}

func (s *service) batteryDrainByBatteryPower(ctx context.Context, b *Broadcaster) {

	chDrain := b.Subscribe(sensorSolakonBatteryPowerEntityID, 1)
	chLevel := b.Subscribe(sensorSolakonBatteryLevelEntityID, 1)
	chBMS := b.Subscribe(string(BMS), 1)

	drain := s.solakon.Initials.BatteryPower
	level := s.solakon.Initials.BatteryLevel

	calculateAndPublish := func() {
		if drain <= 0 || level <= 0 {
			return
		}
		// calculates battery duration in hours
		duration := calculateBatteryDuration(s.solakon.BatteryCapacity, level, s.solakon.BatteryEfficiency, drain)

		s.logger.Debug("Battery duration calculated", slog.Float64("duration", duration))

		s.updateState(HAStatePayload{
			BatteryDuration: strconv.FormatFloat(duration, 'f', 2, 64),
		}, true)
	}

	calculateAndPublish()

	for {
		select {
		case d := <-chDrain:
			drain, _ = strconv.ParseFloat(d.Value, 64)
		case l := <-chLevel:
			level, _ = strconv.ParseFloat(l.Value, 64)
		case <-ctx.Done():
			return
		case <-chBMS: // do run
		}

		calculateAndPublish()
	}

}

func (s *service) batteryMaxDrainUntilLevel(ctx context.Context, b *Broadcaster) {
	chLevel := b.Subscribe(sensorSolakonBatteryLevelEntityID, 1)
	chMinLevel := b.Subscribe(string(MinumumBattery), 1)
	chMinTime := b.Subscribe(string(MinumumBatteryTime), 1)
	chBMS := b.Subscribe(string(BMS), 1)

	level := s.solakon.Initials.BatteryLevel
	minLevel := s.batteryManagement.MinimumBatteryLevel
	// fallback to 00:00:00 if not set
	targetTime := time.Now().Add(24 * time.Hour).Truncate(24 * time.Hour)
	targetTimeParsed, ok := targetTimeFromString(s.batteryManagement.MinimumBatteryTime)
	if ok {
		targetTime = targetTimeParsed
	}

	calculateAndPublish := func() {
		if level < minLevel {
			return
		}
		usableWattHours := s.solakon.BatteryCapacity * 1000 * (level - minLevel) / 100 * s.solakon.BatteryEfficiency

		// targetTime = next day at 10:00 AM
		hoursUntilTarget := targetTime.Sub(time.Now()).Hours()
		maxDrain := usableWattHours / hoursUntilTarget

		s.logger.Debug("Max drain calculated", slog.Float64("max_drain", maxDrain), slog.Time("at", targetTime))

		s.updateState(HAStatePayload{
			MaximumDrainForMinimum: strconv.FormatFloat(maxDrain, 'f', 2, 64),
		}, true)
	}

	calculateAndPublish()

	for {
		select {
		case l := <-chLevel:
			level, _ = strconv.ParseFloat(l.Value, 64)
		case m := <-chMinLevel:
			minLevel, _ = strconv.ParseFloat(m.Value, 64)
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

func targetTimeFromString(timeStr string) (time.Time, bool) {
	// parse time in format "HH:MM:SS"
	parsedTime, err := time.Parse("15:04:05", timeStr)
	if err != nil {
		return time.Time{}, false
	}
	// calculate minTime as duration until the parsed time
	now := time.Now()
	targetTime := time.Date(now.Year(), now.Month(), now.Day(), parsedTime.Hour(), parsedTime.Minute(), parsedTime.Second(), 0, now.Location())
	if targetTime.Before(now) {
		targetTime = targetTime.Add(24 * time.Hour)
	}
	return targetTime, true
}
