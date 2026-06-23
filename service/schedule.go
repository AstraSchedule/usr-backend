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

func parseRuleDate(rule map[string]interface{}) (time.Time, bool) {
	dateStr, _ := rule["date"].(string)
	if dateStr == "" {
		return time.Time{}, false
	}
	d, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return time.Time{}, false
	}
	return d, true
}

func sameDate(a, b time.Time) bool {
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}

func collectCandidates(records []dbTable.AutorunRecord, etype int, school, grade, classNumber string, targetDate time.Time) []scheduleRuleCandidate {
	out := make([]scheduleRuleCandidate, 0)
	for _, r := range records {
		if r.EType != etype {
			continue
		}
		rule := getRule(r.Parameters)
		ruleDate, ok := parseRuleDate(rule)
		if !ok || !sameDate(ruleDate, targetDate) {
			continue
		}
		spec := bestRowSpecificity(r.Scope, school, grade, classNumber)
		if spec < 0 {
			continue
		}
		out = append(out, scheduleRuleCandidate{
			Level: r.Level,
			Spec:  spec,
			Rule:  rule,
		})
	}
	sort.Slice(out, func(i, j int) bool {
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
	classList := make([][]string, 0, len(periods))
	for _, p := range periods {
		classList = append(classList, []string{p.Subject})
	}
	schedule[todayIdx].ClassList = classList
}

func ApplyScheduleRules(base [7]dbTable.DailyClass, timetable map[string]map[string]interface{}, records []dbTable.AutorunRecord, school, grade, classNumber string, targetDate time.Time) [7]dbTable.DailyClass {
	resolved := base
	todayIdx := weekdayIndex(targetDate)

	for _, c := range collectCandidates(records, 0, school, grade, classNumber, targetDate) {
		useDateStr, _ := c.Rule["useDate"].(string)
		useDate, err := time.Parse("2006-01-02", useDateStr)
		if err != nil {
			continue
		}
		srcIdx := weekdayIndex(useDate)
		resolved[todayIdx].ClassList = append([][]string(nil), resolved[srcIdx].ClassList...)
		resolved[todayIdx].Timetable = resolved[srcIdx].Timetable
	}

	for _, c := range collectCandidates(records, 1, school, grade, classNumber, targetDate) {
		timetableID, _ := c.Rule["timetableId"].(string)
		if timetableID == "" {
			continue
		}
		resolved[todayIdx].Timetable = timetableID
	}

	for _, c := range collectCandidates(records, 2, school, grade, classNumber, targetDate) {
		applyPeriodsToDay(&resolved, todayIdx, c.Rule)
	}

	for _, c := range collectCandidates(records, 3, school, grade, classNumber, targetDate) {
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
	start, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		return 1
	}
	days := int(now.Sub(start).Hours() / 24)
	if days < 0 {
		return 1
	}
	return days/7 + 1
}

// ResolveClassList 根据当前周数解析 classList
// classList 格式为 [["数", "语"], ["政"], ["史", "地", "物"]]
// 每个内层数组代表该节课的多周轮换选项
// 返回扁平的 []string，供客户端直接使用
func ResolveClassList(classList [][]string, weekNumber int) []string {
	if len(classList) == 0 {
		return []string{}
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
