package dbTable

type TemperatureStop struct {
	Temp  float64 `json:"temp"`
	Color string  `json:"color"`
}

type TemperatureColorsConfig struct {
	UseGradient bool              `json:"use_gradient"`
	Stops       []TemperatureStop `json:"stops"`
}

type ClientConfigItems struct {
	CountdownTarget      string                   `json:"countdown_target"`
	WeatherAlertOverride bool                     `json:"weather_alert_override"`
	WeatherAlertBrief    bool                     `json:"weather_alert_brief"`
	WeekDisplay          bool                     `json:"week_display"`
	BannerText           string                   `json:"banner_text"`
	CSSStyle             map[string]string        `json:"css_style" gorm:"type:json;not null;serializer:json"`
	TemperatureColors    TemperatureColorsConfig  `json:"temperature_colors" gorm:"type:json;serializer:json"`
}

type ClientConfig struct {
	ID     uint   `gorm:"primaryKey;autoIncrement;not null"`
	School string `gorm:"uniqueIndex:idx_client_configs_school_grade_class,priority:1;not null;size:50"`
	Grade  string `gorm:"uniqueIndex:idx_client_configs_school_grade_class,priority:2;not null;size:50"`
	Class  string `gorm:"uniqueIndex:idx_client_configs_school_grade_class,priority:3;not null;size:50"`
	ClientConfigItems
}
