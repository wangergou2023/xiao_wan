package intent

import (
	"encoding/json"

	"github.com/wangergou2023/xiao_wan/chipper/pkg/logger"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vars"
)

type robotSettingsJSON struct {
	ButtonWakeword int  `json:"button_wakeword"`
	Clock24Hour    bool `json:"clock_24_hour"`
	CustomEyeColor struct {
		Enabled    bool    `json:"enabled"`
		Hue        float64 `json:"hue"`
		Saturation float64 `json:"saturation"`
	} `json:"custom_eye_color"`
	DefaultLocation  string `json:"default_location"`
	DistIsMetric     bool   `json:"dist_is_metric"`
	EyeColor         int    `json:"eye_color"`
	Locale           string `json:"locale"`
	MasterVolume     int    `json:"master_volume"`
	TempIsFahrenheit bool   `json:"temp_is_fahrenheit"`
	TimeZone         string `json:"time_zone"`
}

// loadBotSpeechSettings 从 jdoc 中读取与语音理解相关的机器人设置。
func loadBotSpeechSettings(botSerial, defaultLocation, defaultUnits string) (string, string) {
	botJdoc, jdocExists := vars.GetJdoc("vic:"+botSerial, "vic.RobotSettings")
	if !jdocExists {
		return defaultLocation, defaultUnits
	}

	var robotSettings robotSettingsJSON
	if err := json.Unmarshal([]byte(botJdoc.JsonDoc), &robotSettings); err != nil {
		logger.Println("Error unmarshaling json in paramchecker")
		logger.Println(err)
		return defaultLocation, defaultUnits
	}

	location := robotSettings.DefaultLocation
	units := "C"
	if robotSettings.TempIsFahrenheit {
		units = "F"
	}
	if location == "" {
		location = defaultLocation
	}
	return location, units
}
