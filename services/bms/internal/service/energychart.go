package service

import (
	"context"
	"log/slog"
	"time"
)

func (s *service) energyChartPricing(ctx context.Context, b *Broadcaster) {
	go s.energyChartTodayForcast(ctx)
	go s.energyChartCurrentPrice(ctx, b)
}

func (s *service) energyChartCurrentPrice(ctx context.Context, b *Broadcaster) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	calculateCurrentPrice := func() {
		res, err := s.energyCharts.CurrentPrice(ctx, "")
		if err != nil {
			s.logger.Error("Failed to get current energy chart price", slog.Any("err", err))
			return
		}
		// process the result here
		val, ok := res.Data[0].Values["day_ahead_price"]
		if !ok || val == nil {
			s.logger.Error("Failed to get current energy chart price value", slog.Any("res", res))
			return
		}
		valueEurKWh := *val / 1000.0 // convert from EUR/MWh to EUR/kWh
		value := formatFloat(valueEurKWh)
		s.logger.Debug("Current energy chart price", slog.Any("value", value))
		b.Publish(StateUpdate{ID: string(EnergyCostCurrent), Value: value})
		s.updateState(HAStatePayload{
			EnergyCostCurrent: value,
		}, false)
	}

	calculateCurrentPrice()
	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Done", slog.Any("err", ctx.Err()))
			return
		case <-ticker.C:
			calculateCurrentPrice()
		}
	}
}

func (s *service) energyChartTodayForcast(ctx context.Context) {
	s.logger.Debug("Starting energy chart pricing service")
	// call the energy chart api to get the pricing data
	// get the current forcast
	todayCast, err := s.energyCharts.Price(ctx, "", "", "")
	if err != nil {
		s.logger.Error("Failed to get energy chart pricing", slog.Any("err", err))
		return
	}
	s.logger.Debug("Energy chart pricing data", slog.Any("todayCast.series", todayCast.Series))
	// convert the pricing data to a forecast data structure
	forecastData := &ForcastData{
		Forecast: []*ForcastPoint{},
	}

	for _, cast := range todayCast.Data {

		val := cast.Values["day_ahead_price"]
		timestamp := cast.Timestamp
		s.logger.Debug("Energy chart pricing data point", slog.Any("timestamp", timestamp), slog.Any("value", *val))
		if val == nil {
			continue
		}
		valueEurKWh := *val / 1000.0 // convert from EUR/MWh to EUR/kWh
		value := formatFloat(valueEurKWh)

		forecastData.Forecast = append(forecastData.Forecast, &ForcastPoint{
			DateTime: timestamp,
			Value:    value,
		})

		// interval := time.Duration(todayCast.IntervalMinutes) * time.Minute
		// // check if now is between timestamp and timestamp + interval
		// now := time.Now().UTC()
		// startTime, err := time.Parse(time.RFC3339, timestamp)
		// if err != nil {
		// 	continue
		// }
		// endTime := startTime.UTC().Add(interval)
		// if now.After(startTime) && now.Before(endTime) {
		// 	currentPrice = value
		// }
	}

	s.logger.Info("Energy chart pricing data point", slog.Any("forecastData", forecastData.Forecast))
	// get the current price
	// get the lowest price for the day
	// get the lowest price for the next day 08:00 (may night load is cheaper)

	//	 publish the pricing data to the mqtt topic
	s.updateState(HAStatePayload{
		EnergyCostForecast: forecastData,
	}, false)
}
