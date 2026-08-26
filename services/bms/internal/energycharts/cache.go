package energycharts

import (
	"fmt"
	"math"
	"sync"
	"time"
)

type PriceForecastCache struct {
	mux            *sync.RWMutex
	UpdatedAt      *time.Time
	DayAheadPrices []*DayAheadPrice
}

type DayAheadPrice struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

func NewPriceForecastCache() *PriceForecastCache {
	return &PriceForecastCache{
		mux:            &sync.RWMutex{},
		UpdatedAt:      nil,
		DayAheadPrices: []*DayAheadPrice{},
	}
}

func (c *PriceForecastCache) Insert(key string, priceData []PriceData) {
	c.mux.Lock()
	defer c.mux.Unlock()

	prices := make([]*DayAheadPrice, 0, len(priceData))
	for _, data := range priceData {
		val, ok := data.Values[key]
		if !ok || val == nil {
			continue
		}

		timestamp, err := time.Parse(time.RFC3339, data.Timestamp)
		if err != nil {
			continue
		}
		prices = append(prices, &DayAheadPrice{
			Timestamp: timestamp,
			Value:     *val / 1000.0, // convert from EUR/MWh to EUR/kWh
		})
	}

	c.DayAheadPrices = prices
	now := time.Now()
	c.UpdatedAt = &now
}

// GetLowestPriceForDuration returns the time window with the lowest average price
// for the requested duration. Each entry represents a 15-minute slot.
func (c *PriceForecastCache) GetLowestPriceForDuration(duration time.Duration) (time.Time, time.Time, float64, error) {
	c.mux.RLock()
	defer c.mux.RUnlock()

	if duration <= 0 {
		return time.Time{}, time.Time{}, 0, fmt.Errorf("duration must be positive")
	}
	if len(c.DayAheadPrices) == 0 {
		return time.Time{}, time.Time{}, 0, nil
	}

	step := 15 * time.Minute
	if duration < step {
		return time.Time{}, time.Time{}, 0, fmt.Errorf("duration must be at least 15 minutes")
	}

	windowSlots := int(duration / step)
	if duration%step != 0 {
		windowSlots++
	}
	if windowSlots > len(c.DayAheadPrices) {
		windowSlots = len(c.DayAheadPrices)
	}

	bestStart := time.Time{}
	bestEnd := time.Time{}
	bestAvg := 0.0
	found := false

	for i := 0; i+windowSlots <= len(c.DayAheadPrices); i++ {
		windowSum := 0.0
		for _, price := range c.DayAheadPrices[i : i+windowSlots] {
			windowSum += price.Value
		}
		avg := windowSum / float64(windowSlots)
		if !found || avg < bestAvg {
			bestStart = c.DayAheadPrices[i].Timestamp
			bestEnd = c.DayAheadPrices[i+windowSlots-1].Timestamp.Add(step)
			bestAvg = avg
			found = true
		}
	}

	if !found {
		return time.Time{}, time.Time{}, 0, nil
	}

	return bestStart, bestEnd, bestAvg, nil
}

// GetBestChargingSpots returns the cheapest charging windows needed to reach
// chargeToReach kWh at chargingPower W. Each slot represents 15 minutes.
func (c *PriceForecastCache) GetBestChargingSpots(chargeToReach float64, chargingPower float64) ([][2]time.Time, error) {
	c.mux.RLock()
	defer c.mux.RUnlock()

	if chargeToReach <= 0 {
		return nil, fmt.Errorf("chargeToReach must be positive")
	}
	if chargingPower <= 0 {
		return nil, fmt.Errorf("chargingPower must be positive")
	}
	if len(c.DayAheadPrices) == 0 {
		return nil, nil
	}

	slotDuration := 15 * time.Minute
	slotEnergyKWh := (chargingPower * slotDuration.Hours()) / 1000.0
	if slotEnergyKWh <= 0 {
		return nil, fmt.Errorf("chargingPower is too low for a 15-minute slot")
	}

	remaining := chargeToReach
	used := make([]bool, len(c.DayAheadPrices))
	spots := make([][2]time.Time, 0)

	for remaining > 0 {
		requiredSlots := int(math.Ceil(remaining / slotEnergyKWh))
		if requiredSlots <= 0 {
			requiredSlots = 1
		}
		if requiredSlots > len(c.DayAheadPrices) {
			requiredSlots = len(c.DayAheadPrices)
		}

		bestStart := -1
		bestEnd := -1
		bestAvg := math.Inf(1)

		for i := 0; i+requiredSlots <= len(c.DayAheadPrices); i++ {
			if anyUsed(used, i, i+requiredSlots) {
				continue
			}

			windowSum := 0.0
			for _, price := range c.DayAheadPrices[i : i+requiredSlots] {
				windowSum += price.Value
			}
			avg := windowSum / float64(requiredSlots)
			if avg < bestAvg {
				bestAvg = avg
				bestStart = i
				bestEnd = i + requiredSlots - 1
			}
		}

		if bestStart < 0 || bestEnd < 0 {
			break
		}

		for i := bestStart; i <= bestEnd; i++ {
			used[i] = true
		}

		start := c.DayAheadPrices[bestStart].Timestamp
		end := c.DayAheadPrices[bestEnd].Timestamp.Add(slotDuration)
		spots = append(spots, [2]time.Time{start, end})
		remaining -= float64(requiredSlots) * slotEnergyKWh
	}

	return spots, nil
}

func anyUsed(used []bool, start, end int) bool {
	for i := start; i < end; i++ {
		if i >= 0 && i < len(used) && used[i] {
			return true
		}
	}
	return false
}
