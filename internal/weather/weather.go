package weather

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"
)

// Default location: Copenhagen, Denmark
var (
	latitude  = 55.6761
	longitude = 12.5683
)

func init() {
	if v := os.Getenv("HEST_LATITUDE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			latitude = f
		}
	}
	if v := os.Getenv("HEST_LONGITUDE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			longitude = f
		}
	}
}

// WMO Weather interpretation codes mapped to emoji
var wmoEmoji = map[int]string{
	0:  "☀️",  // Clear sky
	1:  "🌤️", // Mainly clear
	2:  "⛅",   // Partly cloudy
	3:  "☁️",  // Overcast
	45: "🌫️", // Fog
	48: "🌫️", // Depositing rime fog
	51: "🌦️", // Light drizzle
	53: "🌦️", // Moderate drizzle
	55: "🌧️", // Dense drizzle
	56: "🌧️", // Light freezing drizzle
	57: "🌧️", // Dense freezing drizzle
	61: "🌧️", // Slight rain
	63: "🌧️", // Moderate rain
	65: "🌧️", // Heavy rain
	66: "🌧️", // Light freezing rain
	67: "🌧️", // Heavy freezing rain
	71: "🌨️", // Slight snow fall
	73: "🌨️", // Moderate snow fall
	75: "🌨️", // Heavy snow fall
	77: "🌨️", // Snow grains
	80: "🌧️", // Slight rain showers
	81: "🌧️", // Moderate rain showers
	82: "🌧️", // Violent rain showers
	85: "🌨️", // Slight snow showers
	86: "🌨️", // Heavy snow showers
	95: "⛈️", // Thunderstorm
	96: "⛈️", // Thunderstorm with slight hail
	99: "⛈️", // Thunderstorm with heavy hail
}

type response struct {
	Daily struct {
		WeatherCode []int     `json:"weather_code"`
		Temperature []float64 `json:"temperature_2m_max"`
	} `json:"daily"`
}

// Fetch returns a weather emoji and temperature string for the given date.
// Location defaults to Copenhagen, Denmark but can be overridden with
// HEST_LATITUDE and HEST_LONGITUDE environment variables.
func Fetch(date time.Time, client *http.Client) (string, error) {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}

	dateStr := date.Format("2006-01-02")
	url := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%.4f&longitude=%.4f&daily=weather_code,temperature_2m_max&timezone=auto&start_date=%s&end_date=%s",
		latitude, longitude, dateStr, dateStr,
	)

	return fetchFromURL(url, client, date)
}

func fetchFromURL(url string, client *http.Client, date time.Time) (string, error) {
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("weather request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("weather API returned status %d", resp.StatusCode)
	}

	var data response
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", fmt.Errorf("decoding weather response: %w", err)
	}

	if len(data.Daily.WeatherCode) == 0 || len(data.Daily.Temperature) == 0 {
		return "", fmt.Errorf("no weather data for %s", date.Format("2006-01-02"))
	}

	code := data.Daily.WeatherCode[0]
	temp := data.Daily.Temperature[0]

	emoji, ok := wmoEmoji[code]
	if !ok {
		emoji = "🌡️"
	}

	return fmt.Sprintf("%s %.0f°", emoji, temp), nil
}
