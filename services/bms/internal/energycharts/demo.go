package energycharts

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
)

// LoadPriceResponseFromFile reads a JSONC fixture file such as
// _ignore_/response_price.jsonc and unmarshals it into PriceResponse.
func LoadPriceResponseFromFile(path string) (*PriceResponse, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cleaned := stripJSONC(string(data))
	var resp PriceResponse
	if err := json.Unmarshal([]byte(cleaned), &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

func stripJSONC(input string) string {
	input = strings.ReplaceAll(input, "\r\n", "\n")

	lineCommentRE := regexp.MustCompile(`(?m)\s*//.*$`)
	input = lineCommentRE.ReplaceAllString(input, "")

	blockCommentRE := regexp.MustCompile(`(?s)/\*.*?\*/`)
	input = blockCommentRE.ReplaceAllString(input, "")

	trailingCommaRE := regexp.MustCompile(`,\s*([}\]])`)
	input = trailingCommaRE.ReplaceAllString(input, "$1")

	return strings.TrimSpace(input)
}
