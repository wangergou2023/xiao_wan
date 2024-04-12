package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	config "github.com/wangergou2023/xiao_wan/chipper/plugins/xiao_wan/config"
	plugins "github.com/wangergou2023/xiao_wan/chipper/plugins/xiao_wan/plugins"
	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

var Plugin plugins.Plugin = &WeatherPlugin{}

type WeatherPlugin struct {
	cfg          config.Cfg
	openaiClient *openai.Client
}

func (w *WeatherPlugin) Init(cfg config.Cfg, openaiClient *openai.Client) error {
	w.cfg = cfg
	w.openaiClient = openaiClient
	return nil
}

func (w WeatherPlugin) ID() string {
	return "weather"
}

func (w WeatherPlugin) Description() string {
	return "获取指定地点的当前天气情况。"
}

func (w WeatherPlugin) FunctionDefinition() openai.FunctionDefinition {
	return openai.FunctionDefinition{
		Name:        "weather",
		Description: "返回指定地点的当前天气情况。",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"location": {
					Type:        jsonschema.String,
					Description: "查询天气的地点。",
				},
			},
			Required: []string{"location"},
		},
	}
}

func (w WeatherPlugin) Execute(jsonInput string) (string, error) {
	var input struct {
		Location string `json:"location"`
	}
	err := json.Unmarshal([]byte(jsonInput), &input)
	if err != nil {
		return "", err
	}

	weatherInfo, err := w.getWeather(input.Location)
	if err != nil {
		return "", err
	}

	return weatherInfo, nil
}

func (w WeatherPlugin) getWeather(location string) (string, error) {
	// 使用配置中的API密钥
	apiKey := w.cfg.OpenWeatherMapAPIKey()
	url := fmt.Sprintf("http://api.openweathermap.org/data/2.5/weather?q=%s&appid=%s&units=metric", location, apiKey)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result struct {
		Weather []struct {
			Description string `json:"description"`
		} `json:"weather"`
		Main struct {
			Temp float64 `json:"temp"`
		} `json:"main"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if len(result.Weather) > 0 {
		return fmt.Sprintf("%s, 温度: %.1f°C", result.Weather[0].Description, result.Main.Temp), nil
	}

	return "未能获取天气信息", nil
}
