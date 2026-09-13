package dbTable

import "time"

// 自动任务规则类型（与 API 契约中的 type 数值一致，禁止改动数值）
const (
	AutorunTypeCompensation = 0 // 调休
	AutorunTypeTimetable    = 1 // 作息表调整
	AutorunTypeSchedule     = 2 // 课程表调整
	AutorunTypeAll          = 3 // 全部调整
	AutorunTypeClientConfig = 4 // 客户端配置（自动套用桌面端本地设置）
)

// 生效条件类型（AutorunCondition.Kind）
const (
	AutorunWhenDate   = "date"   // 单日
	AutorunWhenRange  = "range"  // 日期范围（含首尾）
	AutorunWhenWeekly = "weekly" // 每 N 周轮换
	AutorunWhenEvent  = "event"  // 时刻事件（节次开始/结束等）
	AutorunWhenCron   = "cron"   // cron 表达式
)

// 时刻事件类型（AutorunCondition.Event）
const (
	AutorunEventStartup    = "startup"     // 客户端启动
	AutorunEventClassStart = "class_start" // 第 N 节课开始
	AutorunEventClassEnd   = "class_end"   // 第 N 节课结束
)

// AutorunCondition 条目生效条件。
// 所有条件都会被归一化成一段时间区间求值（见 service.EntryWindow），
// 因此判定是幂等的：客户端离线一段时间后再上线，也能正确算出「此刻该不该生效」。
type AutorunCondition struct {
	Kind       string `json:"kind"`
	Date       string `json:"date,omitempty"`       // kind=date：单日
	StartDate  string `json:"startDate,omitempty"`  // kind=range/weekly：下界，weekly 同时用作周期锚点
	EndDate    string `json:"endDate,omitempty"`    // 上界（含当天），留空表示不设终点
	EveryWeeks int    `json:"everyWeeks,omitempty"` // kind=weekly：每 N 周
	WeekOffset int    `json:"weekOffset,omitempty"` // kind=weekly：周期内第几个槽位（0 起）
	Weekdays   []int  `json:"weekdays,omitempty"`   // 限定星期（0=周日 … 6=周六），空为不限
	Event      string `json:"event,omitempty"`      // kind=event
	Period     int    `json:"period,omitempty"`     // kind=event：第几节课（1 起）
	Cron       string `json:"cron,omitempty"`       // kind=cron：5 字段表达式
	Duration   int    `json:"duration,omitempty"`   // kind=cron：命中后持续分钟数，0 表示持续到下一次命中
}

// AutorunEntry 任务内的一条条目：生效条件 + 内容。
// When 为 nil 表示无条件，在任务生效域内始终生效。
type AutorunEntry struct {
	ID       string                 `json:"id"`
	Disabled bool                   `json:"disabled,omitempty"`
	Note     string                 `json:"note,omitempty"`
	When     *AutorunCondition      `json:"when,omitempty"`
	Action   map[string]interface{} `json:"action"`
}

// AutorunRecord 一条记录即一个自动任务：任务级字段（名称/类型/生效域/优先级/启停）+ 若干条目。
// Parameters 为 v1 遗留的单条规则（{rule: {...}}）；当 Entries 为空且 Parameters 有值时，
// 读取侧会把它合成为一条单日条目，旧数据无需迁移即可继续工作。
type AutorunRecord struct {
	HashID     string                 `gorm:"primaryKey;not null;size:64" json:"hashid"`
	Name       string                 `gorm:"size:128" json:"name"`
	EType      int                    `gorm:"not null;index" json:"etype"`
	Scope      []string               `gorm:"type:json;not null;serializer:json" json:"scope"`
	Disabled   bool                   `gorm:"not null;default:false" json:"disabled"`
	Entries    []AutorunEntry         `gorm:"type:json;serializer:json" json:"entries"`
	Parameters map[string]interface{} `gorm:"type:json;not null;serializer:json" json:"parameters"`
	Level      int                    `gorm:"not null" json:"level"`
	Status     int                    `gorm:"not null" json:"status"`
	CreatedAt  time.Time              `json:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at"`
}
