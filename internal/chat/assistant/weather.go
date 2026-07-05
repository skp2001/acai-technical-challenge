package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/openai/openai-go/v2"
)

type weatherResponse struct {
	Location struct {
		Name string `json:"name"`
	} `json:"location"`

	Current struct {
		TempC     float64 `json:"temp_c"`
		WindKph   float64 `json:"wind_kph"`
		Humidity  int     `json:"humidity"`
		Condition struct {
			Text string `json:"text"`
		} `json:"condition"`
	} `json:"current"`
}

// GetWeather fetches the current weather for the given location using WeatherAPI.
func getWeather(ctx context.Context, location string) (string, error) {
	apiKey := os.Getenv("WEATHER_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("WEATHER_API_KEY is not set")
	}

	apiURL := fmt.Sprintf(
		"http://api.weatherapi.com/v1/current.json?key=%s&q=%s",
		apiKey,
		url.QueryEscape(location),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var weather weatherResponse
	if err := json.NewDecoder(resp.Body).Decode(&weather); err != nil {
		return "", err
	}

	return fmt.Sprintf(
		"Location: %s\nTemperature: %.1f°C\nCondition: %s\nWind: %.1f km/h\nHumidity: %d%%",
		weather.Location.Name,
		weather.Current.TempC,
		weather.Current.Condition.Text,
		weather.Current.WindKph,
		weather.Current.Humidity,
	), nil
}

// WeatherTool implements the Tool interface for fetching weather information.
type WeatherTool struct{}

func (t *WeatherTool) Name() string {
	return "get_weather"
}

func (t *WeatherTool) Definition() openai.ChatCompletionToolUnionParam {
	return openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
		Name:        "get_weather",
		Description: openai.String("Get weather at the given location"),
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"location": map[string]string{
					"type": "string",
				},
			},
			"required": []string{"location"},
		},
	})
}

func (t *WeatherTool) Execute(ctx context.Context, arguments string) (string, error) {
	var payload struct {
		Location string `json:"location"`
	}

	if err := json.Unmarshal([]byte(arguments), &payload); err != nil {
		return "", fmt.Errorf("failed to parse weather arguments: %w", err)
	}

	return getWeather(ctx, payload.Location)
}
