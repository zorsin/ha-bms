package energycharts

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/zorsin/ha-bms/services/bms/internal/config"
)

const defaultBaseURL = "https://api.energy-charts.info/v2"

type Options struct {
	Config     *config.Config
	Logger     *slog.Logger
	HTTPClient *http.Client
}

type Client struct {
	cfg        *config.Config
	logger     *slog.Logger
	httpClient *http.Client
	baseURL    string
}

func New(opts Options) *Client {
	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	baseURL := opts.Config.EnergyChartsURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	return &Client{
		cfg:        opts.Config,
		logger:     opts.Logger,
		httpClient: httpClient,
		baseURL:    strings.TrimRight(baseURL, "/"),
	}
}

// Price returns day-ahead prices for the requested bidding zone and interval.
// Empty parameters use the API defaults.
func (c *Client) Price(ctx context.Context, biddingZone, start, end string) (*PriceResponse, error) {
	query := url.Values{}
	if biddingZone != "" {
		query.Set("bzn", biddingZone)
	}
	if start != "" {
		query.Set("start", start)
	}
	if end != "" {
		query.Set("end", end)
	}
	return c.getPrice(ctx, "price", query)
}

// CurrentPrice returns the price for the current market interval.
func (c *Client) CurrentPrice(ctx context.Context, biddingZone string) (*PriceResponse, error) {
	return c.getPrice(ctx, "price_current", biddingZoneQuery(biddingZone))
}

// NextDayPrice returns day-ahead prices for the next local day of a bidding zone.
func (c *Client) NextDayPrice(ctx context.Context, biddingZone string) (*PriceResponse, error) {
	return c.getPrice(ctx, "price_next_day", biddingZoneQuery(biddingZone))
}

func biddingZoneQuery(biddingZone string) url.Values {
	query := url.Values{}
	if biddingZone != "" {
		query.Set("bzn", biddingZone)
	}
	return query
}

func (c *Client) getPrice(ctx context.Context, endpoint string, query url.Values) (*PriceResponse, error) {
	requestURL := c.baseURL + "/" + endpoint
	if encodedQuery := query.Encode(); encodedQuery != "" {
		requestURL += "?" + encodedQuery
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create Energy-Charts %s request: %w", endpoint, err)
	}
	request.Header.Set("Accept", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("request Energy-Charts %s: %w", endpoint, err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4<<10))
		return nil, fmt.Errorf("Energy-Charts %s returned %s: %s", endpoint, response.Status, strings.TrimSpace(string(body)))
	}

	var price PriceResponse
	if err := json.NewDecoder(response.Body).Decode(&price); err != nil {
		return nil, fmt.Errorf("decode Energy-Charts %s response: %w", endpoint, err)
	}
	return &price, nil
}
