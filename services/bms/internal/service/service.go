package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	ha "github.com/mkelcik/go-ha-client/v2"
	"github.com/zorsin/ha-bms/services/bms/internal/config"
)

type service struct {
	cfg      *config.Config
	logger   *slog.Logger
	haClient *ha.Client
	haWs     *ha.WSClient
	solakon  *SolakonOne
}

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
	// get initial states
	// subscribe to HA events
	s.startSubscriptions(ctx, b)

	go s.batteryDrainByBatteryPower(ctx, b)
}

func (s *service) getStaticStates(ctx context.Context) bool {
	solakon := &SolakonOne{
		BatteryEfficiency: 0.85,
		ACMaxAllowedPower: 800,
	}
	totalOk := true
	state, err := s.haClient.GetStateForEntity(ctx, sensorSolakonBatteryCapacityEntityID)
	if err != nil {
		totalOk = false
	}
	solakon.BatteryCapacity, _ = strconv.ParseFloat(state.State, 64)

	state, err = s.haClient.GetStateForEntity(ctx, sensorSolakonBatteryLevelEntityID)
	if err != nil {
		totalOk = false
	}
	solakon.Initials.BatteryLevel, _ = strconv.ParseFloat(state.State, 64)
	state, err = s.haClient.GetStateForEntity(ctx, sensorSolakonBatteryPowerEntityID)
	if err != nil {
		totalOk = false
	}
	solakon.Initials.BatteryPower, _ = strconv.ParseFloat(state.State, 64)

	s.solakon = solakon
	s.logger.Info("solakon state", slog.Any("solakon", solakon))
	return totalOk
}

func (s *service) batteryDrainByBatteryPower(ctx context.Context, b *Broadcaster) {

	chDrain := b.Subscribe(sensorSolakonBatteryPowerEntityID, 1)
	chLevel := b.Subscribe(sensorSolakonBatteryLevelEntityID, 1)

	drain := s.solakon.Initials.BatteryPower
	level := s.solakon.Initials.BatteryLevel

	for {
		select {
		case d := <-chDrain:
			drain, _ = strconv.ParseFloat(d.Value, 64)
		case l := <-chLevel:
			level, _ = strconv.ParseFloat(l.Value, 64)
		}

		if drain > 0 && level > 0 {
			// calculates battery duration in hours
			duration := calculateBatteryDuration(s.solakon.BatteryCapacity, level, s.solakon.BatteryEfficiency, drain)

			s.logger.Info("Battery duration calculated", slog.Float64("duration", duration))

			s.setEntityValue(ctx, sensorBmsBatteryDurationEntityID, ha.State{
				State: strconv.FormatFloat(duration, 'f', 2, 64),
				Attributes: map[string]interface{}{
					"unique_id":           "uid_bms_battery_duration",
					"unit_of_measurement": "h",
					"device_class":        "duration",
					"friendly_name":       "BMS Battery Duration",
					"state_class":         "measurement",
				},
			})
		}
	}

}

func calculateBatteryDuration(batteryCapacity, batteryLevel, batteryEfficiency, drain float64) float64 {

	batteryAvailable := batteryLevel / 100 * batteryCapacity
	batteryDuration := (batteryAvailable * 1000 * batteryEfficiency) / drain

	return batteryDuration
}

func (s *service) setEntityValue(ctx context.Context, entityID string, state ha.State) error {
	_, err := s.haClient.CreateState(ctx, entityID, state)
	if err != nil {
		s.logger.Error("Failed to set Home Assistant entity state",
			slog.String("entity_id", entityID),
			slog.Any("err", err),
		)
		return err
	}

	return nil
}
