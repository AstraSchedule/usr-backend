package service

import (
	"AstraScheduleServerGo/model/dbTable"
	"strings"
	"time"
)

// autorunDateFormat 自动任务条件中的日期格式
const autorunDateFormat = "2006-01-02"

// RuleContext 自动任务条件求值上下文
type RuleContext struct {
	Now       time.Time // 求值时刻
	TermStart string    // 学期起始日期（YYYY-MM-DD），周期条件按它推算学期周次
}

// dateOnly 去掉时分秒，仅保留日期
func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func parseConditionDate(value string, location *time.Location) (time.Time, bool) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, false
	}
	d, err := time.ParseInLocation(autorunDateFormat, strings.TrimSpace(value), location)
	if err != nil {
		return time.Time{}, false
	}
	return dateOnly(d), true
}

// EntriesOf 返回记录的全部条目。
// 仅为兼容 v1 数据：当 Entries 为空而 Parameters 带有一条单日规则时，
// 合成为「一条单日条目」，旧记录无需迁移即可被新引擎处理。
func EntriesOf(r dbTable.AutorunRecord) []dbTable.AutorunEntry {
	if len(r.Entries) > 0 {
		out := make([]dbTable.AutorunEntry, 0, len(r.Entries))
		for _, e := range r.Entries {
			if len(e.Action) == 0 {
				continue
			}
			out = append(out, e)
		}
		return out
	}
	rule := getRule(r.Parameters)
	if len(rule) == 0 {
		return nil
	}
	action := make(map[string]interface{}, len(rule))
	for k, v := range rule {
		if k == "date" {
			continue
		}
		action[k] = v
	}
	if len(action) == 0 {
		return nil
	}
	entry := dbTable.AutorunEntry{ID: r.HashID, Action: action}
	if dateStr, _ := rule["date"].(string); dateStr != "" {
		entry.When = &dbTable.AutorunCondition{Kind: dbTable.AutorunWhenDate, Date: dateStr}
	}
	return []dbTable.AutorunEntry{entry}
}

// MatchEntry 判断条目在当前上下文是否命中
func MatchEntry(e dbTable.AutorunEntry, ctx RuleContext) bool {
	if e.Disabled {
		return false
	}
	return MatchCondition(e.When, ctx)
}

// MatchCondition 判断条件在当前上下文是否命中。
// nil 条件表示无条件命中（任务生效域内始终生效）。
func MatchCondition(when *dbTable.AutorunCondition, ctx RuleContext) bool {
	if when == nil {
		return true
	}
	now := ctx.Now
	if !withinConditionBounds(when, now) {
		return false
	}
	if !weekdayAllowed(when, now) {
		return false
	}
	if when.EveryWeeks > 0 && !matchWeekCycle(when, ctx) {
		return false
	}
	switch when.Kind {
	case "", dbTable.AutorunWhenDate:
		// 日期相等性已在 withinConditionBounds 中判定
		return true
	case dbTable.AutorunWhenRange, dbTable.AutorunWhenWeekly, dbTable.AutorunWhenEvent:
		return true
	case dbTable.AutorunWhenCron:
		start, end, ok := cronWindow(when, now)
		return ok && !now.Before(start) && now.Before(end)
	default:
		return false
	}
}

// withinConditionBounds 判定日期上下界：kind=date 用 date，其余用 startDate/endDate（含首尾）
func withinConditionBounds(when *dbTable.AutorunCondition, now time.Time) bool {
	today := dateOnly(now)
	if when.Kind == "" || when.Kind == dbTable.AutorunWhenDate {
		day, ok := parseConditionDate(when.Date, now.Location())
		return ok && sameDate(day, today)
	}
	if start, ok := parseConditionDate(when.StartDate, now.Location()); ok && today.Before(start) {
		return false
	}
	if end, ok := parseConditionDate(when.EndDate, now.Location()); ok && today.After(end) {
		return false
	}
	return true
}

func weekdayAllowed(when *dbTable.AutorunCondition, now time.Time) bool {
	if len(when.Weekdays) == 0 {
		return true
	}
	current := int(now.Weekday())
	for _, v := range when.Weekdays {
		if v == current {
			return true
		}
	}
	return false
}

// matchWeekCycle 判定「每 N 周的第 X 周」：周期锚点优先取条件自身的 startDate，
// 其次取学期起始日；周次沿用 CalcWeekNumber（周一切分）。
func matchWeekCycle(when *dbTable.AutorunCondition, ctx RuleContext) bool {
	every := when.EveryWeeks
	if every <= 0 {
		every = 1
	}
	anchor := strings.TrimSpace(when.StartDate)
	if anchor == "" {
		anchor = strings.TrimSpace(ctx.TermStart)
	}
	if anchor == "" {
		// 没有学期起始日时无法推算周次，退化为「第 1 周」语义
		return when.WeekOffset%every == 0
	}
	week := CalcWeekNumber(anchor, ctx.Now)
	if week < 1 {
		week = 1
	}
	offset := when.WeekOffset % every
	if offset < 0 {
		offset += every
	}
	return (week-1)%every == offset
}

// cronWindow 把 cron 条件展开成生效区间：
// 起点为不晚于 now 的上一次命中；终点为「起点 + duration 分钟」，未填 duration 时取下一次命中。
func cronWindow(when *dbTable.AutorunCondition, now time.Time) (time.Time, time.Time, bool) {
	spec, ok := ParseCron(when.Cron)
	if !ok {
		return time.Time{}, time.Time{}, false
	}
	start, ok := spec.Prev(now)
	if !ok {
		return time.Time{}, time.Time{}, false
	}
	if when.Duration > 0 {
		return start, start.Add(time.Duration(when.Duration) * time.Minute), true
	}
	if next, ok := spec.Next(now); ok {
		return start, next, true
	}
	return start, start.Add(24 * time.Hour), true
}

// ClientConfigRule 下发给桌面端的客户端配置规则（由桌面端按条件本地调度）
type ClientConfigRule struct {
	TaskID      string                    `json:"taskId"`
	TaskName    string                    `json:"name,omitempty"`
	EntryID     string                    `json:"entryId"`
	Priority    int                       `json:"priority"`
	Scope       string                    `json:"scope"`
	Specificity int                       `json:"specificity"`
	When        *dbTable.AutorunCondition `json:"when,omitempty"`
	Settings    map[string]interface{}    `json:"settings"`
}

// CollectClientConfigRules 收集对指定班级生效的客户端配置条目。
// 服务端只按生效域过滤，时间条件留给桌面端本地求值（这样客户端离线也不会漏事件）。
func CollectClientConfigRules(records []dbTable.AutorunRecord, school, grade, classNumber string) []ClientConfigRule {
	out := make([]ClientConfigRule, 0)
	for _, r := range records {
		if r.EType != dbTable.AutorunTypeClientConfig || r.Disabled {
			continue
		}
		spec, scope := bestRowSpecificityAndScope(r.Scope, school, grade, classNumber)
		if spec < 0 {
			continue
		}
		for _, entry := range EntriesOf(r) {
			if entry.Disabled {
				continue
			}
			settings, ok := entry.Action["settings"].(map[string]interface{})
			if !ok || len(settings) == 0 {
				continue
			}
			out = append(out, ClientConfigRule{
				TaskID:      r.HashID,
				TaskName:    r.Name,
				EntryID:     entry.ID,
				Priority:    r.Level,
				Scope:       scope,
				Specificity: spec,
				When:        entry.When,
				Settings:    settings,
			})
		}
	}
	return out
}

// EntryWindow 返回条目在时间轴上的生效区间（用于任务状态推导）。
// start/end 为半开区间；hasStart/hasEnd 为 false 表示该侧不设界（长期生效）。
func EntryWindow(e dbTable.AutorunEntry, ctx RuleContext) (start, end time.Time, hasStart, hasEnd bool) {
	when := e.When
	if when == nil {
		return time.Time{}, time.Time{}, false, false
	}
	location := ctx.Now.Location()
	switch when.Kind {
	case "", dbTable.AutorunWhenDate:
		day, ok := parseConditionDate(when.Date, location)
		if !ok {
			return time.Time{}, time.Time{}, false, false
		}
		return day, day.AddDate(0, 0, 1), true, true
	default:
		if s, ok := parseConditionDate(when.StartDate, location); ok {
			start, hasStart = s, true
		}
		if en, ok := parseConditionDate(when.EndDate, location); ok {
			end, hasEnd = en.AddDate(0, 0, 1), true
		}
		return start, end, hasStart, hasEnd
	}
}

// TaskWindow 汇总任务内所有条目的生效区间（并集）。
func TaskWindow(r dbTable.AutorunRecord, ctx RuleContext) (start, end time.Time, hasStart, hasEnd bool) {
	for _, e := range EntriesOf(r) {
		s, en, hs, he := EntryWindow(e, ctx)
		if hs && (!hasStart || s.Before(start)) {
			start, hasStart = s, true
		}
		if he && (!hasEnd || en.After(end)) {
			end, hasEnd = en, true
		}
	}
	return start, end, hasStart, hasEnd
}

// TaskStatus 按任务整体生效区间推导状态：0 待生效 / 1 生效中 / 2 已过期
func TaskStatus(r dbTable.AutorunRecord, today time.Time) int {
	if len(EntriesOf(r)) == 0 {
		return 0
	}
	day := dateOnly(today)
	start, end, hasStart, hasEnd := TaskWindow(r, RuleContext{Now: today})
	if hasStart && day.Before(start) {
		return 0
	}
	if hasEnd && !day.Before(end) {
		return 2
	}
	return 1
}
