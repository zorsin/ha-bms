package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	ha "github.com/mkelcik/go-ha-client/v2"
	"github.com/zorsin/ha-bms/services/bms/internal/config"
	"github.com/zorsin/ha-bms/services/bms/internal/energycharts"
)

type service struct {
	cfg               *config.Config
	logger            *slog.Logger
	haClient          *ha.Client
	haWs              *ha.WSClient
	mqttClient        mqtt.Client
	solakon           *SolakonOne
	batteryManagement *BatteryManagement
	energyCharts      *energycharts.Client
}

const (
	initializationWaitTime = 5 * time.Second
)

func New(cfg *config.Config, logger *slog.Logger) *service {

	client, err := ha.NewClient(cfg.HAURL, ha.WithToken(cfg.HAToken), ha.WithTimeout(30*time.Second))
	if err != nil {
		logger.Error("Failed to create HA client", slog.Any("err", err))
		return nil
	}

	haWs := client.WS(
		ha.WithAutoReconnect(true),
		ha.WithMaxRetries(0),
		ha.WithReconnectBackoff(time.Second, 30*time.Second),
		ha.WithOnReconnect(func() {
			logger.Info("Reconnected to HA WebSocket")
		}),
		ha.WithOnReconnectError(func(err error) {
			logger.Warn("Reconnect attempt failed", slog.Any("err", err))
		}),
	)
	srv := &service{
		cfg:      cfg,
		logger:   logger,
		haClient: client,
		haWs:     haWs,
		energyCharts: energycharts.New(energycharts.Options{
			Config: cfg,
			Logger: logger,
		}),
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

		b := NewBroadcaster()
		s.energyChartDemo(ctx, b)
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
	go s.energyChartPricing(ctx, b)
}
