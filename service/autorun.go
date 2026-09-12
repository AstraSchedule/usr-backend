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

// VersionBoundary 返回该班课表配置「下一次可能变化」的时刻（Unix 秒），0 表示不存在后续变化点。
// 客户端只需比较版本串：变化点之前一直命中 304；越过变化点后版本必然不同，下一次请求即拿到新配置。
// 已过期或长期不变的条目不会再产生变化点，因此不会无谓地破坏缓存。
func VersionBoundary(records []dbTable.AutorunRecord, school, grade, classNumber string, now time.Time) int64 {
	ctx := RuleContext{Now: now}
	boundary := int64(0)
	for _, r := range records {
		if r.Disabled || r.EType == dbTable.AutorunTypeClientConfig {
			continue
		}
		if bestRowSpecificity(r.Scope, school, grade, classNumber) < 0 {
			continue
		}
		for _, entry := range EnabledEntriesOf(r) {
			next := entryNextBoundary(entry, ctx)
			if next == 0 {
				continue
			}
			if boundary == 0 || next < boundary {
				boundary = next
			}
		}
	}
	return boundary
}

// entryNextBoundary 计算条目下一次改变命中结果的时刻；0 表示不会再变化
func entryNextBoundary(e dbTable.AutorunEntry, ctx RuleContext) int64 {
	when := e.When
	if when == nil {
		return 0
	}
	switch when.Kind {
	case dbTable.AutorunWhenDate:
		return dateNextBoundary(when, ctx)
	case dbTable.AutorunWhenRange:
		return rangeNextBoundary(when, ctx)
	case dbTable.AutorunWhenCron:
		return cronNextBoundary(when, ctx)
	case dbTable.AutorunWhenWeekly:
		return weeklyNextBoundary(when, ctx)
	default:
		return 0
	}
}

// dateNextBoundary 单日条件：未来日期从当日零点开始生效，当天则到次日零点结束
func dateNextBoundary(when *dbTable.AutorunCondition, ctx RuleContext) int64 {
	day, ok := parseConditionDate(when.Date, ctx.Now.Location())
	if !ok {
		return 0
	}
	if ctx.Now.Before(day) {
		return day.Unix()
	}
	end := day.AddDate(0, 0, 1)
	if ctx.Now.Before(end) {
		return end.Unix()
	}
	return 0
}

// rangeNextBoundary 日期范围：未来起点开始生效；有终点的在终点次日失效；无终点的不会再变化
func rangeNextBoundary(when *dbTable.AutorunCondition, ctx RuleContext) int64 {
	location := ctx.Now.Location()
	if start, ok := parseConditionDate(when.StartDate, location); ok && ctx.Now.Before(start) {
		return start.Unix()
	}
	end, ok := parseConditionDate(when.EndDate, location)
	if !ok {
		return 0
	}
	endExclusive := end.AddDate(0, 0, 1)
	if ctx.Now.Before(endExclusive) {
		return endExclusive.Unix()
	}
	return 0
}

func cronNextBoundary(when *dbTable.AutorunCondition, ctx RuleContext) int64 {
	spec, ok := ParseCron(when.Cron)
	if !ok {
		return 0
	}
	next, ok := spec.Next(ctx.Now)
	if !ok {
		return 0
	}
	return next.Unix()
}

// weeklyNextBoundary 纯周次条件只在周切换时变化（周次已写进版本串）；限定星期时每天零点变化一次
func weeklyNextBoundary(when *dbTable.AutorunCondition, ctx RuleContext) int64 {
	if len(when.Weekdays) == 0 {
		return 0
	}
	location := ctx.Now.Location()
	if end, ok := parseConditionDate(when.EndDate, location); ok && !ctx.Now.Before(end.AddDate(0, 0, 1)) {
		return 0
	}
	if start, ok := parseConditionDate(when.StartDate, location); ok && ctx.Now.Before(start) {
		return start.Unix()
	}
	return dateOnly(ctx.Now).AddDate(0, 0, 1).Unix()
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

// EnabledEntriesOf 返回启用中的条目（停用条目不参与解析与状态推导）
func EnabledEntriesOf(r dbTable.AutorunRecord) []dbTable.AutorunEntry {
	all := EntriesOf(r)
	out := make([]dbTable.AutorunEntry, 0, len(all))
	for _, e := range all {
		if e.Disabled {
			continue
		}
		out = append(out, e)
	}
	return out
}

// TaskStatus 逐个启用条目判定状态：0 待生效 / 1 生效中 / 2 已过期。
// 不能用「最早起点 + 最晚终点」的包络区间代替：9/1 与 9/10 两条单日条目之间的 9/5 并不生效。
func TaskStatus(r dbTable.AutorunRecord, today time.Time) int {
	entries := EnabledEntriesOf(r)
	if len(entries) == 0 {
		return 0
	}
	ctx := RuleContext{Now: today}
	hasUpcoming := false
	for _, e := range entries {
		start, end, hasStart, hasEnd := EntryWindow(e, ctx)
		started := !hasStart || !today.Before(start)
		ended := hasEnd && !today.Before(end)
		if started && !ended {
			return 1
		}
		if !ended {
			hasUpcoming = true
		}
	}
	if hasUpcoming {
		return 0
	}
	return 2
}
