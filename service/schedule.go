package service

import (
	"AstraScheduleServerGo/model/dbTable"
	"sort"
	"strconv"
	"strings"
	"time"
)

type scheduleRuleCandidate struct {
	Level int
	Spec  int
	Type  int
	Rule  map[string]interface{}
}

type Period struct {
	No      int    `json:"no"`
	Subject string `json:"subject"`
}

func weekdayIndex(d time.Time) int {
	return int(d.Weekday())
}

func scopeSpecificity(scopeEntry, school, grade, classNumber string) int {
	s := strings.TrimSpace(scopeEntry)
	if s == "" {
		return -1
	}
	if strings.EqualFold(s, "ALL") {
		return 0
	}
	ctx := []string{school, grade, classNumber}
	parts := strings.Split(s, "/")
	if len(parts) > len(ctx) {
		return -1
	}
	for i := range parts {
		if parts[i] != ctx[i] {
			return -1
		}
	}
	return len(parts)
}

// bestRowSpecificityAndScope 返回最具体的作用域条目及其具体度（-1 表示全部不匹配）
func bestRowSpecificityAndScope(scope []string, school, grade, classNumber string) (int, string) {
	best := -1
	bestScope := ""
	for _, s := range scope {
		spec := scopeSpecificity(s, school, grade, classNumber)
		if spec > best {
			best = spec
			bestScope = s
		}
	}
	return best, bestScope
}

func bestRowSpecificity(scope []string, school, grade, classNumber string) int {
	best := -1
	for _, s := range scope {
		spec := scopeSpecificity(s, school, grade, classNumber)
		if spec > best {
			best = spec
		}
	}
	return best
}

func getRule(params map[string]interface{}) map[string]interface{} {
	if params == nil {
		return map[string]interface{}{}
	}
	if rule, ok := params["rule"].(map[string]interface{}); ok {
		return rule
	}
	return params
}

func sameDate(a, b time.Time) bool {
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}

// collectCandidates 收集指定类型下所有命中的条目。
// 一条任务可以带多条条目，命中判定统一走 service 的条件引擎（MatchEntry）。
func collectCandidates(records []dbTable.AutorunRecord, etype int, school, grade, classNumber string, ctx RuleContext) []scheduleRuleCandidate {
	return collectCandidatesMulti(records, []int{etype}, school, grade, classNumber, ctx)
}

// collectCandidatesMulti 收集多个类型的命中条目，供「调课」与「课程表调整」同层竞合使用：
// 两类条目按用户设置的优先级（Level）混排，同优先级时保持录入顺序。
func collectCandidatesMulti(records []dbTable.AutorunRecord, etypes []int, school, grade, classNumber string, ctx RuleContext) []scheduleRuleCandidate {
	wanted := make(map[int]struct{}, len(etypes))
	for _, etype := range etypes {
		wanted[etype] = struct{}{}
	}
	out := make([]scheduleRuleCandidate, 0)
	for _, r := range records {
		if r.Disabled {
			continue
		}
		if _, ok := wanted[r.EType]; !ok {
			continue
		}
		spec := bestRowSpecificity(r.Scope, school, grade, classNumber)
		if spec < 0 {
			continue
		}
		for _, entry := range EntriesOf(r) {
			if !MatchEntry(entry, ctx) {
				continue
			}
			out = append(out, scheduleRuleCandidate{
				Level: r.Level,
				Spec:  spec,
				Type:  r.EType,
				Rule:  entry.Action,
			})
		}
	}
	// 稳定排序：同优先级同作用域时保持录入顺序（同一任务内多条条目先后生效）
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Level == out[j].Level {
			return out[i].Spec < out[j].Spec
		}
		return out[i].Level < out[j].Level
	})
	return out
}

func asInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case float32:
		return int(n), true
	case float64:
		return int(n), true
	case string:
		i, err := strconv.Atoi(n)
		if err != nil {
			return 0, false
		}
		return i, true
	default:
		return 0, false
	}
}

func applyPeriodsToDay(schedule *[7]dbTable.DailyClass, todayIdx int, rule map[string]interface{}) {
	scheduleObj, ok := rule["schedule"].(map[string]interface{})
	if !ok {
		return
	}
	periodsRaw, ok := scheduleObj["periods"].([]interface{})
	if !ok {
		return
	}
	type pair struct {
		No      int
		Subject string
	}
	periods := make([]pair, 0)
	for _, raw := range periodsRaw {
		item, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		no, ok := asInt(item["no"])
		if !ok || no <= 0 {
			continue
		}
		subject, _ := item["subject"].(string)
		periods = append(periods, pair{No: no, Subject: subject})
	}
	sort.Slice(periods, func(i, j int) bool { return periods[i].No < periods[j].No })
	classList := make(dbTable.ClassList, 0, len(periods))
	for _, p := range periods {
		classList = append(classList, []string{p.Subject})
	}
	schedule[todayIdx].ClassList = classList
}

// swapSide 调课的一端：某一天的某一节（节次从 1 开始）
type swapSide struct {
	Date   time.Time
	Period int
}

// parseSwapSide 解析调课的一端，任一项非法时返回 false
func parseSwapSide(raw interface{}) (swapSide, bool) {
	obj, ok := raw.(map[string]interface{})
	if !ok {
		return swapSide{}, false
	}
	dateStr, _ := obj["date"].(string)
	date, err := time.ParseInLocation("2006-01-02", dateStr, time.Local)
	if err != nil {
		return swapSide{}, false
	}
	period, ok := asInt(obj["period"])
	if !ok || period <= 0 {
		return swapSide{}, false
	}
	return swapSide{Date: date, Period: period}, true
}

// swapSubjectAt 取某一天某一节当前解析后的科目（按该日期所在周解析每周轮换课表）
func swapSubjectAt(day dbTable.DailyClass, period int, weekNumber int) (string, bool) {
	subjects := ResolveClassList(day.ClassList, weekNumber)
	if period < 1 || period > len(subjects) {
		return "", false
	}
	return subjects[period-1], true
}

// setSubjectAt 覆盖某一天某一节的科目；节次越界时不做任何修改。
// 写入前先复制一份 ClassList：resolved 与调用方传入的 base 共享底层数组，
// 原地写入会污染调用方数据，导致同一份 base 的后续解析看到上一次的结果。
func setSubjectAt(day *dbTable.DailyClass, period int, subject string) {
	if period < 1 || period > len(day.ClassList) {
		return
	}
	classList := make(dbTable.ClassList, len(day.ClassList))
	copy(classList, day.ClassList)
	classList[period-1] = []string{subject}
	day.ClassList = classList
}

// swapPeriodsInDay 交换同一天内两节课的科目；同一节或节次越界时不做任何修改
func swapPeriodsInDay(day *dbTable.DailyClass, first, second, weekNumber int) {
	if first == second {
		return
	}
	subjects := ResolveClassList(day.ClassList, weekNumber)
	if first < 1 || second < 1 || first > len(subjects) || second > len(subjects) {
		return
	}
	firstSubject, secondSubject := subjects[first-1], subjects[second-1]
	setSubjectAt(day, first, secondSubject)
	setSubjectAt(day, second, firstSubject)
}

// applySwapRule 应用一条调课：把两节具体的课互换。
// 同一天内的互换直接交换两节；跨天互换时只改写「当前请求日期」这一端（与其它规则一致，
// 另一端在它自己那天被请求时改写），对方科目按对方日期所在周解析，因此每周轮换课表也能取到正确科目。
func applySwapRule(resolved *[7]dbTable.DailyClass, rule map[string]interface{}, ctx RuleContext) {
	swapObj, ok := rule["swap"].(map[string]interface{})
	if !ok {
		return
	}
	from, fromOK := parseSwapSide(swapObj["from"])
	to, toOK := parseSwapSide(swapObj["to"])
	if !fromOK || !toOK {
		return
	}
	todayIdx := weekdayIndex(ctx.Now)
	if sameDate(from.Date, to.Date) {
		if !sameDate(ctx.Now, from.Date) {
			return
		}
		swapPeriodsInDay(&resolved[todayIdx], from.Period, to.Period, CalcWeekNumber(ctx.TermStart, ctx.Now))
		return
	}
	cur, other := from, to
	if sameDate(ctx.Now, to.Date) {
		cur, other = to, from
	} else if !sameDate(ctx.Now, from.Date) {
		return
	}
	subject, ok := swapSubjectAt(resolved[weekdayIndex(other.Date)], other.Period, CalcWeekNumber(ctx.TermStart, other.Date))
	if !ok || subject == "" {
		return
	}
	setSubjectAt(&resolved[todayIdx], cur.Period, subject)
}

// ApplyScheduleRules 兼容入口：只按日期匹配（无学期起始日，周期条件按第 1 周语义求值）。
// 新调用方请使用 ApplyScheduleRulesCtx 以便传入学期起始日等上下文。
func ApplyScheduleRules(base [7]dbTable.DailyClass, timetable map[string]map[string]interface{}, records []dbTable.AutorunRecord, school, grade, classNumber string, targetDate time.Time) [7]dbTable.DailyClass {
	return ApplyScheduleRulesCtx(base, timetable, records, school, grade, classNumber, RuleContext{Now: targetDate})
}

// ApplyScheduleRulesCtx 按 COMPENSATION → TIMETABLE → (SCHEDULE + 调课) → ALL 的顺序叠加自动任务条目。
// 其中「调课」与「课程表调整」在同一层按优先级混排。
func ApplyScheduleRulesCtx(base [7]dbTable.DailyClass, timetable map[string]map[string]interface{}, records []dbTable.AutorunRecord, school, grade, classNumber string, ctx RuleContext) [7]dbTable.DailyClass {
	resolved := base
	todayIdx := weekdayIndex(ctx.Now)

	for _, c := range collectCandidates(records, 0, school, grade, classNumber, ctx) {
		useDateStr, _ := c.Rule["useDate"].(string)
		useDate, err := time.Parse("2006-01-02", useDateStr)
		if err != nil {
			continue
		}
		srcIdx := weekdayIndex(useDate)
		resolved[todayIdx].ClassList = append(dbTable.ClassList(nil), resolved[srcIdx].ClassList...)
		resolved[todayIdx].Timetable = resolved[srcIdx].Timetable
	}

	for _, c := range collectCandidates(records, 1, school, grade, classNumber, ctx) {
		timetableID, _ := c.Rule["timetableId"].(string)
		if timetableID == "" {
			continue
		}
		resolved[todayIdx].Timetable = timetableID
	}

	// 调课（互换两节具体的课）与课程表调整同层竞合：按用户设置的优先级 level 混排，
	// 同优先级保持录入顺序，最后生效的条目胜出。
	for _, c := range collectCandidatesMulti(records, []int{dbTable.AutorunTypeSchedule, dbTable.AutorunTypeLessonSwap}, school, grade, classNumber, ctx) {
		if c.Type == dbTable.AutorunTypeLessonSwap {
			applySwapRule(&resolved, c.Rule, ctx)
			continue
		}
		applyPeriodsToDay(&resolved, todayIdx, c.Rule)
	}

	for _, c := range collectCandidates(records, 3, school, grade, classNumber, ctx) {
		timetableID, _ := c.Rule["timetableId"].(string)
		if timetableID != "" {
			resolved[todayIdx].Timetable = timetableID
		}
		applyPeriodsToDay(&resolved, todayIdx, c.Rule)
	}

	FixWrongTimetable(&resolved, timetable)
	return resolved
}

func firstTimetableKey(timetable map[string]map[string]interface{}) string {
	if _, ok := timetable["常日"]; ok {
		return "常日"
	}
	return "常日"
}

func timetableNeedCount(timetable map[string]map[string]interface{}, timetableID string) int {
	items, ok := timetable[timetableID]
	if !ok {
		return 0
	}
	maxIdx := -1
	for _, v := range items {
		i, ok := asInt(v)
		if !ok {
			continue
		}
		if i > maxIdx {
			maxIdx = i
		}
	}
	if maxIdx < 0 {
		return 0
	}
	return maxIdx + 1
}

func FixWrongTimetable(schedule *[7]dbTable.DailyClass, timetable map[string]map[string]interface{}) {
	fallback := firstTimetableKey(timetable)
	for i := 0; i < len(schedule); i++ {
		day := &schedule[i]
		if _, ok := timetable[day.Timetable]; !ok {
			day.Timetable = fallback
		}
		need := timetableNeedCount(timetable, day.Timetable)
		if need == 0 {
			continue
		}
		if len(day.ClassList) > need {
			day.ClassList = day.ClassList[:need]
			continue
		}
		if len(day.ClassList) < need {
			for len(day.ClassList) < need {
				day.ClassList = append(day.ClassList, []string{"课"})
			}
		}
	}
}

// CalcWeekNumber 根据开学日期和当前日期计算当前是第几周（从1开始）
func CalcWeekNumber(startDateStr string, now time.Time) int {
	if startDateStr == "" {
		return 1
	}
	location := now.Location()
	start, err := time.ParseInLocation("2006-01-02", startDateStr, location)
	if err != nil {
		return 1
	}
	// 周数按周一切分：开学日期所在周为第 1 周，而不是从开学日连续计数 7 天。
	// 将归一化后的日期放到 UTC 再计算，避免本地时区/DST 造成小时差偏移。
	startMonday := mondayDateUTC(start)
	currentMonday := mondayDateUTC(now.In(location))
	days := int(currentMonday.Sub(startMonday).Hours() / 24)
	if days < 0 {
		return 1
	}
	return days/7 + 1
}

func mondayDateUTC(date time.Time) time.Time {
	date = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	daysSinceMonday := (int(date.Weekday()) + 6) % 7
	date = date.AddDate(0, 0, -daysSinceMonday)
	return time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
}

// ResolveClassList 根据当前周数解析 classList
// classList 格式为 [["数", "语"], ["政"], ["史", "地", "物"]]
// 每个内层数组代表该节课的多周轮换选项
// 返回扁平的 []string，供客户端直接使用
func ResolveClassList(classList dbTable.ClassList, weekNumber int) []string {
	if len(classList) == 0 {
		return []string{}
	}
	if weekNumber < 1 {
		weekNumber = 1
	}
	resolved := make([]string, 0, len(classList))
	for _, item := range classList {
		if len(item) == 0 {
			resolved = append(resolved, "")
		} else if len(item) == 1 {
			resolved = append(resolved, item[0])
		} else {
			// 多周轮换：按周数索引取值（从1开始，所以用 weekNumber-1）
			idx := (weekNumber - 1) % len(item)
			resolved = append(resolved, item[idx])
		}
	}
	return resolved
}

func BuildPeriodsForDate(schedule [7]dbTable.DailyClass, timetable map[string]map[string]interface{}, date time.Time) []Period {
	idx := weekdayIndex(date)
	day := schedule[idx]
	tb := timetable[day.Timetable]
	indicesMap := map[int]struct{}{}
	for _, v := range tb {
		i, ok := asInt(v)
		if !ok {
			continue
		}
		indicesMap[i] = struct{}{}
	}
	indices := make([]int, 0, len(indicesMap))
	for i := range indicesMap {
		indices = append(indices, i)
	}
	sort.Ints(indices)
	out := make([]Period, 0, len(indices))
	for _, i := range indices {
		subject := ""
		if i >= 0 && i < len(day.ClassList) && len(day.ClassList[i]) > 0 {
			subject = day.ClassList[i][0]
		}
		out = append(out, Period{No: i + 1, Subject: subject})
	}
	return out
}
