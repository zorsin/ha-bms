package energycharts

// PriceResponse is returned by the Energy-Charts v2 price endpoint.
type PriceResponse struct {
	SchemaVersion   string         `json:"schema_version"`
	Endpoint        string         `json:"endpoint"`
	Country         string         `json:"country"`
	BiddingZone     string         `json:"bidding_zone"`
	Timezone        string         `json:"timezone"`
	Resolution      string         `json:"resolution"`
	IntervalMinutes int            `json:"interval_minutes"`
	Unit            string         `json:"unit"`
	GeneratedAt     string         `json:"generated_at"`
	AvailableFrom   string         `json:"available_from"`
	AvailableUntil  string         `json:"available_until"`
	Series          []PriceSeries  `json:"series"`
	Data            []PriceData    `json:"data"`
	Attributes      map[string]any `json:"attributes"`
	License         string         `json:"license"`
	Deprecated      bool           `json:"deprecated"`
}

// PriceSeries describes one price value available in each PriceData record.
type PriceSeries struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// PriceData contains price values for one timestamp. Values are keyed by a
// PriceSeries ID, such as "day_ahead_price".
type PriceData struct {
	Timestamp string              `json:"timestamp"`
	Values    map[string]*float64 `json:"values"`
}
