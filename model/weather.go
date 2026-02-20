package model

// WeatherResponse 定义天气响应结构
type WeatherResponse struct {
	Where     string `json:"where"`
	Temp      string `json:"temp"`
	Weat      string `json:"weat"`
	Wind      string `json:"wind"`
	WindPower string `json:"wind_power"`
	Warn      string `json:"warn"`
	BriefWarn string `json:"brief_warn"`
}

// WeatherNow 定义当前天气信息结构
type WeatherNow struct {
	Temp      string `json:"temp"`
	Text      string `json:"text"`
	WindDir   string `json:"windDir"`
	WindScale string `json:"windScale"`
}

// WeatherResp 定义天气响应结构
type WeatherResp struct {
	Now WeatherNow `json:"now"`
}

// LocationResp 定义地理位置响应结构
type LocationResp struct {
	ID   string  `json:"id"`
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
	Name string  `json:"name"`
}

// WarningResp 定义天气预警响应结构
type WarningResp struct {
	Alerts []Alert `json:"alerts"`
}

// Alert 定义预警信息结构
type Alert struct {
	Description string `json:"description"`
	Headline    string `json:"headline"`
}
