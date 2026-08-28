package energycharts

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

const (
	slotDuration = 15 * time.Minute
)

type PriceForecastCache struct {
	mux                    *sync.RWMutex
	UpdatedAt              *time.Time
	DayAheadValidAt        *time.Time
	DayAheadPrices         []*DayAheadPrice
	TomorrowUpdatedAt      *time.Time
	TomorrowDayAheadPrices []*DayAheadPrice
	TomorrowValidAt        *time.Time
}

type DayAheadPrice struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

func NewPriceForecastCache() *PriceForecastCache {
	return &PriceForecastCache{
		mux:                    &sync.RWMutex{},
		UpdatedAt:              nil,
		DayAheadValidAt:        nil,
		DayAheadPrices:         []*DayAheadPrice{},
		TomorrowUpdatedAt:      nil,
		TomorrowDayAheadPrices: []*DayAheadPrice{},
		TomorrowValidAt:        nil,
	}
}

func (c *PriceForecastCache) Insert(key string, priceData []PriceData) {
	c.mux.Lock()
	defer c.mux.Unlock()

	c.DayAheadPrices = toDayAheadPrices(priceData, key)
	c.DayAheadValidAt = &c.DayAheadPrices[0].Timestamp
	now := time.Now()
	c.UpdatedAt = &now
}

func (c *PriceForecastCache) InsertTomorrow(key string, priceData []PriceData) {
	c.mux.Lock()
	defer c.mux.Unlock()

	c.TomorrowDayAheadPrices = toDayAheadPrices(priceData, key)
	c.TomorrowValidAt = &c.TomorrowDayAheadPrices[0].Timestamp
	now := time.Now()
	c.TomorrowUpdatedAt = &now
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
func (c *PriceForecastCache) GetBestChargingSpots(chargeToReach float64, chargingPower float64, start time.Time) ([][2]time.Time, error) {
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

	cutoff := time.Now()
	if start.After(cutoff) {
		cutoff = start
	}
	firstEligible := sort.Search(len(c.DayAheadPrices), func(index int) bool {
		return !c.DayAheadPrices[index].Timestamp.Before(cutoff)
	})
	// spot now active
	prices := c.DayAheadPrices[firstEligible-1:]
	if len(prices) == 0 {
		return nil, nil
	}

	slotEnergyKWh := (chargingPower * slotDuration.Hours()) / 1000.0

	remaining := chargeToReach
	used := make([]bool, len(prices))
	spots := make([][2]time.Time, 0)

	for remaining > 0 {
		requiredSlots := int(math.Ceil(remaining / slotEnergyKWh))
		if requiredSlots <= 0 {
			requiredSlots = 1
		}
		if requiredSlots > len(prices) {
			requiredSlots = len(prices)
		}

		bestStart := -1
		bestEnd := -1
		bestAvg := math.Inf(1)

		for i := 0; i+requiredSlots <= len(prices); i++ {
			if anyUsed(used, i, i+requiredSlots) {
				continue
			}

			windowSum := 0.0
			for _, price := range prices[i : i+requiredSlots] {
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

		start := prices[bestStart].Timestamp
		end := prices[bestEnd].Timestamp.Add(slotDuration)
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

// GetLowestPrice returns the lowest price inside the cache. Returns nil if no prices are available.
func (c *PriceForecastCache) GetLowestPrice() (*DayAheadPrice, error) {
	c.mux.RLock()
	defer c.mux.RUnlock()

	return getLowestPrice(c.DayAheadPrices)
}

func (c *PriceForecastCache) GetLowestPriceTomorrow() (*DayAheadPrice, error) {
	c.mux.RLock()
	defer c.mux.RUnlock()

	return getLowestPrice(c.TomorrowDayAheadPrices)
}

func toDayAheadPrices(prices []PriceData, key string) []*DayAheadPrice {
	dayAheadPrices := make([]*DayAheadPrice, 0, len(prices))
	for _, data := range prices {
		val, ok := data.Values[key]
		if !ok || val == nil {
			continue
		}

		timestamp, err := time.Parse(time.RFC3339, data.Timestamp)
		if err != nil {
			continue
		}
		dayAheadPrices = append(dayAheadPrices, &DayAheadPrice{
			Timestamp: timestamp,
			Value:     *val / 1000.0, // convert from EUR/MWh to EUR/kWh
		})
	}
	// sort by timestamp to ensure the prices are in chronological order
	sort.Slice(dayAheadPrices, func(i, j int) bool {
		return dayAheadPrices[i].Timestamp.Before(dayAheadPrices[j].Timestamp)
	})
	return dayAheadPrices
}

func getLowestPrice(prices []*DayAheadPrice) (*DayAheadPrice, error) {
	if len(prices) == 0 {
		return nil, nil
	}

	lowest := prices[0]
	for _, price := range prices[1:] {
		if price.Value < lowest.Value {
			lowest = price
		}
	}

	return lowest, nil
}
