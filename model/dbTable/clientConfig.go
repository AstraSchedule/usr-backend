package dbTable

type CSSStyle struct {
	CenterFontSize      string `json:"--center-font-size"`
	CornerFontSize      string `json:"--corner-font-size"`
	CountdownFontSize   string `json:"--countdown-font-size"`
	GlobalBorderRadius  string `json:"--global-border-radius"`
	GlobalBgOpacity     string `json:"--global-bg-opacity"`
	ContainerBgPadding  string `json:"--container-bg-padding"`
	CountdownBgPadding  string `json:"--countdown-bg-padding"`
	ContainerSpace      string `json:"--container-space"`
	TopSpace            string `json:"--top-space"`
	MainHorizontalSpace string `json:"--main-horizontal-space"`
	DividerWidth        string `json:"--divider-width"`
	DividerMargin       string `json:"--divider-margin"`
	TriangleSize        string `json:"--triangle-size"`
	SubFontSize         string `json:"--sub-font-size"`
	BannerHeight        string `json:"--banner-height"`
}

type ClientConfigItems struct {
	CountdownTarget string   `json:"countdown_target"`
	WeekDisplay     bool     `json:"week_display"`
	BannerText      string   `json:"banner_text"`
	CSSStyle        CSSStyle `json:"css_style" gorm:"type:json;not null"`
}

type ClientConfig struct {
	ID     uint   `gorm:"primaryKey;autoIncrement;not null"`
	School string `gorm:"index:idx_school_grade_class,priority:1;not null"`
	Grade  string `gorm:"index:idx_school_grade_class,priority:2;not null"`
	Class  string `gorm:"index:idx_school_grade_class,priority:3;not null"`
	ClientConfigItems
}
