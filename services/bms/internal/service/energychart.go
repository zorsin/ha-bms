package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/zorsin/ha-bms/services/bms/internal/energycharts"
)

func (s *service) energyChartDemo(ctx context.Context, b *Broadcaster) {
	s.logger.Info("Starting demo mode")
	// load data from demo json file
	resp, err := energycharts.LoadPriceResponseFromFile("../../_ignore_/response_price.jsonc")
	if err != nil {
		s.logger.Error("Failed to load energy chart demo data", slog.Any("err", err))
		return
	}

	priceForcastCache := energycharts.NewPriceForecastCache()
	priceForcastCache.Insert("day_ahead_price", resp.Data)

	start, end, price, err := priceForcastCache.GetLowestPriceForDuration(2 * time.Hour)
	if err != nil {
		s.logger.Error("Failed to get average price for duration", slog.Any("err", err))
		return
	}

	s.logger.Info("Demo mode: Average price for 1 hour", slog.Any("start", start), slog.Any("end", end), slog.Any("price", price))

	spots, err := priceForcastCache.GetBestChargingSpots(6, 1000)
	if err != nil {
		s.logger.Error("Failed to get best charging spots", slog.Any("err", err))
		return
	}
	s.logger.Info("Demo mode: Best charging spots", slog.Any("spots", spots))
}

func (s *service) energyChartPricing(ctx context.Context, b *Broadcaster) {
	// forcast of the current day & next day
	go s.updateForcasts(ctx)
	// get the current price
	go s.energyChartCurrentPrice(ctx, b)

	// get the lowest price for the day
	// get the lowest price for the next day 08:00 (may night load is cheaper)
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

func (s *service) updateForcasts(ctx context.Context) {

	runForcasts := func() {
		s.logger.Debug("Updating forcasts")
		go s.energyChartTodayForcast(ctx)
		go s.energyChartTomorrowForcast(ctx)
	}

	runForcasts()

	// wait until full hour to update the forcast data
	now := time.Now()
	nextHour := now.Truncate(time.Hour).Add(time.Hour).Add(10 * time.Second)
	timeUntilNextHour := time.Until(nextHour)
	s.logger.Info("Waiting until next hour to update forcasts", slog.Any("timeUntilNextHour", timeUntilNextHour))
	time.Sleep(timeUntilNextHour)
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	// run before the first tick runs in given time
	runForcasts()
	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Done - stopping update forcasts", slog.Any("err", ctx.Err()))
			return
		case <-ticker.C:
			runForcasts()
		}
	}
}

func (s *service) energyChartTodayForcast(ctx context.Context) {
	s.logger.Debug("Starting energy chart pricing service")
	// call the energy chart api to get the pricing data
	// get the current forcast

	todayCast, err := s.energyCharts.Price(ctx, "", time.Now().Format("2006-01-02"), "")
	if err != nil {
		s.logger.Error("Failed to get energy chart pricing", slog.Any("err", err))
		return
	}
	s.logger.Debug("Energy chart pricing data", slog.Any("todayCast.series", todayCast.Series))
	// convert the pricing data to a forecast data structure
	forecastData := s.getForcastData(todayCast)
	s.logger.Debug("Energy chart pricing data point", slog.Any("forecastData", forecastData.Forecast[0]))

	s.priceForecastCache.Insert("day_ahead_price", todayCast.Data)
	lowest, err := s.priceForecastCache.GetLowestPrice()
	if err == nil {
		forecastData.Current = formatFloat(lowest.Value)
	}
	// publish the pricing data to the mqtt topic
	s.updateState(HAStatePayload{
		EnergyCostForecast: forecastData,
	}, false)
}

func (s *service) energyChartTomorrowForcast(ctx context.Context) {
	s.logger.Debug("Starting energy chart pricing service")
	now := time.Now()
	// only between 14:00 and 23:59, get the next day forcast
	if now.Hour() < 14 || now.Hour() > 23 {
		s.logger.Debug("Skipping energychart forcast for tomorrow, not between 14:00 and 23:59", slog.Any("hour", now.Hour()))
		return
	}
	// call the energy chart api to get the pricing data
	// get the current forcast
	tomorrowCast, err := s.energyCharts.NextDayPrice(ctx, "")
	if err != nil {
		s.logger.Error("Failed to get energy chart pricing", slog.Any("err", err))
		return
	}
	s.logger.Debug("Energy chart pricing data", slog.Any("tomorrowCast.series", tomorrowCast.Series))
	// convert the pricing data to a forecast data structure
	forecastData := s.getForcastData(tomorrowCast)
	s.logger.Debug("Energy chart pricing data point", slog.Any("forecastData", forecastData.Forecast[0]))

	s.priceForecastCache.InsertTomorrow("day_ahead_price", tomorrowCast.Data)
	lowest, err := s.priceForecastCache.GetLowestPriceTomorrow()
	if err == nil {
		forecastData.Current = formatFloat(lowest.Value)
	}
	// publish the pricing data to the mqtt topic
	s.updateState(HAStatePayload{
		EnergyCostForecastTomorrow: forecastData,
	}, false)
}

func (s *service) getForcastData(input *energycharts.PriceResponse) *ForcastData {
	forecastData := &ForcastData{
		Forecast: []*ForcastPoint{},
	}

	for _, cast := range input.Data {
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
	}
	return forecastData
}
