package web

import "AstraScheduleServerGo/model/dbTable"

type textItem struct {
	Text string `json:"text"`
}

type subjectsPayload struct {
	Abbr     []textItem `json:"abbr"`
	FullName []textItem `json:"fullName"`
}

type dailyClassInput struct {
	Chinese   string     `json:"Chinese"`
	English   string     `json:"English"`
	ClassList [][]string `json:"classList"`
	Timetable string     `json:"timetable"`
}

type schedulePayload struct {
	DailyClass []dailyClassInput `json:"daily_class"`
}

type autorunPayload struct {
	Type     int                    `json:"type"`
	Scope    interface{}            `json:"scope"`
	Priority int                    `json:"priority"`
	ID       string                 `json:"id"`
	Content  map[string]interface{} `json:"content"`
}

// autorunEntryInput 统一任务接口中的条目；enabled 缺省为 true
type autorunEntryInput struct {
	ID      string                    `json:"id"`
	Enabled *bool                     `json:"enabled"`
	Note    string                    `json:"note"`
	When    *dbTable.AutorunCondition `json:"when"`
	Action  map[string]interface{}    `json:"action"`
}

// autorunTaskPayload 统一任务接口载荷：一个任务 = 任务级字段 + 若干条目
type autorunTaskPayload struct {
	ID       string              `json:"id"`
	Name     string              `json:"name"`
	Type     int                 `json:"type"`
	Scope    interface{}         `json:"scope"`
	Priority int                 `json:"priority"`
	Enabled  *bool               `json:"enabled"`
	Entries  []autorunEntryInput `json:"entries"`
}

type countdownScheduleInput struct {
	Name     string `json:"name"`
	Date     string `json:"date"`
	Priority int    `json:"priority"`
}

type countdownPayload struct {
	ID        string                   `json:"id"`
	Scope     interface{}              `json:"scope"`
	Schedules []countdownScheduleInput `json:"schedules"`
}

type copyScopePayload struct {
	School      string `json:"school"`
	Grade       string `json:"grade"`
	Class       string `json:"class"`
	ClassNumber string `json:"class_number"`
}

func (s copyScopePayload) ClassValue() string {
	if s.Class != "" {
		return s.Class
	}
	return s.ClassNumber
}

type copyConfigPayload struct {
	From copyScopePayload `json:"from"`
	To   copyScopePayload `json:"to"`
}
