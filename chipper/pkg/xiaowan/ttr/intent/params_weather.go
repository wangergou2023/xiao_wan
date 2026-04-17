package intent

import "github.com/wangergou2023/xiao_wan/chipper/pkg/xiaowan/ttr/support"

// buildWeatherParamsFromSpeech 将天气口语查询转成 intent 参数，并返回是否需要走错误播报。
func buildWeatherParamsFromSpeech(speechText, botLocation, botUnits string) (map[string]string, bool) {
	condition, isForecast, localDatetime, speakableLocation, temperature, temperatureUnit := support.WeatherParser(speechText, botLocation, botUnits)
	if localDatetime == "test" {
		return nil, true
	}
	return map[string]string{
		"condition":                 condition,
		"is_forecast":               isForecast,
		"local_datetime":            localDatetime,
		"speakable_location_string": speakableLocation,
		"temperature":               temperature,
		"temperature_unit":          temperatureUnit,
	}, false
}

// buildWeatherParamsFromSlots 为 slot 模式构造固定格式的天气参数。
func buildWeatherParamsFromSlots(botLocation, botUnits string) map[string]string {
	condition, isForecast, localDatetime, speakableLocation, temperature, temperatureUnit := support.WeatherParser("what's the weather", botLocation, botUnits)
	return map[string]string{
		"condition":                 condition,
		"is_forecast":               isForecast,
		"local_datetime":            localDatetime,
		"speakable_location_string": speakableLocation,
		"temperature":               temperature,
		"temperature_unit":          temperatureUnit,
	}
}
