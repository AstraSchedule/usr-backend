package service

import (
	"strconv"
	"strings"
	"time"
)

// 最小 5 字段 cron 实现（分 时 日 月 周），不引第三方依赖。
// 支持 "*"、单值、范围 "a-b"、步长 "*/n" 与 "a-b/n"、以及逗号列表。
// 星期 0 表示周日（与 time.Weekday 一致）；日与周同时受限时按标准 cron 的「或」语义。

const (
	cronFieldCount = 5
	cronMaxDays    = 366
)

var cronFieldRanges = [cronFieldCount][2]int{
	{0, 59}, // 分
	{0, 23}, // 时
	{1, 31}, // 日
	{1, 12}, // 月
	{0, 6},  // 周
}

type cronField struct {
	any    bool
	values map[int]bool
}

type cronSpec struct {
	minute  cronField
	hour    cronField
	day     cronField
	month   cronField
	weekday cronField
}

// ParseCron 解析 5 字段 cron 表达式，非法表达式返回 false。
func ParseCron(expr string) (cronSpec, bool) {
	fields := strings.Fields(strings.TrimSpace(expr))
	if len(fields) != cronFieldCount {
		return cronSpec{}, false
	}
	spec := cronSpec{}
	targets := []*cronField{&spec.minute, &spec.hour, &spec.day, &spec.month, &spec.weekday}
	for i, raw := range fields {
		parsed, ok := parseCronField(raw, cronFieldRanges[i][0], cronFieldRanges[i][1])
		if !ok {
			return cronSpec{}, false
		}
		*targets[i] = parsed
	}
	return spec, true
}

// IsValidCron 校验 5 字段 cron 表达式是否合法
func IsValidCron(expr string) bool {
	_, ok := ParseCron(expr)
	return ok
}

func parseCronField(raw string, minValue, maxValue int) (cronField, bool) {
	field := cronField{values: map[int]bool{}}
	if raw == "*" {
		field.any = true
		return field, true
	}
	for _, part := range strings.Split(raw, ",") {
		if part == "" {
			return cronField{}, false
		}
		step := 1
		body := part
		if idx := strings.Index(part, "/"); idx >= 0 {
			n, err := strconv.Atoi(part[idx+1:])
			if err != nil || n <= 0 {
				return cronField{}, false
			}
			step = n
			body = part[:idx]
		}
		lo, hi := minValue, maxValue
		if body != "*" {
			bounds := strings.SplitN(body, "-", 2)
			start, err := strconv.Atoi(strings.TrimSpace(bounds[0]))
			if err != nil {
				return cronField{}, false
			}
			lo, hi = start, start
			if len(bounds) == 2 {
				end, err := strconv.Atoi(strings.TrimSpace(bounds[1]))
				if err != nil {
					return cronField{}, false
				}
				hi = end
			}
		}
		if lo < minValue || hi > maxValue || lo > hi {
			return cronField{}, false
		}
		for v := lo; v <= hi; v += step {
			field.values[v] = true
		}
	}
	return field, true
}

func (f cronField) match(value int) bool {
	if f.any {
		return true
	}
	return f.values[value]
}

func (s cronSpec) matchDay(t time.Time) bool {
	if !s.month.match(int(t.Month())) {
		return false
	}
	dayOK := s.day.match(t.Day())
	weekdayOK := s.weekday.match(int(t.Weekday()))
	switch {
	case s.day.any && s.weekday.any:
		return true
	case s.day.any:
		return weekdayOK
	case s.weekday.any:
		return dayOK
	default:
		return dayOK || weekdayOK
	}
}

func (s cronSpec) match(t time.Time) bool {
	return s.matchDay(t) && s.hour.match(t.Hour()) && s.minute.match(t.Minute())
}

// Prev 返回不晚于 t 的上一次命中时刻（秒被截断到整分）。
func (s cronSpec) Prev(t time.Time) (time.Time, bool) {
	limit := t.Truncate(time.Minute)
	day := time.Date(limit.Year(), limit.Month(), limit.Day(), 0, 0, 0, 0, limit.Location())
	for i := 0; i <= cronMaxDays; i++ {
		if s.matchDay(day) {
			if hit, ok := s.lastHitOfDay(day, limit); ok {
				return hit, true
			}
		}
		day = day.AddDate(0, 0, -1)
	}
	return time.Time{}, false
}

// Next 返回严格晚于 t 的下一次命中时刻。
func (s cronSpec) Next(t time.Time) (time.Time, bool) {
	limit := t.Truncate(time.Minute)
	day := time.Date(limit.Year(), limit.Month(), limit.Day(), 0, 0, 0, 0, limit.Location())
	for i := 0; i <= cronMaxDays; i++ {
		if s.matchDay(day) {
			if hit, ok := s.firstHitOfDay(day, limit, i > 0); ok {
				return hit, true
			}
		}
		day = day.AddDate(0, 0, 1)
	}
	return time.Time{}, false
}

func (s cronSpec) lastHitOfDay(day, limit time.Time) (time.Time, bool) {
	// 只有与 limit 同一天时才需要按 limit 截断，更早的日子整天都可取（从 23 时开始倒推）
	onLimitDay := sameDate(day, limit)
	startHour := 23
	if onLimitDay {
		startHour = limit.Hour()
	}
	for hour := startHour; hour >= 0; hour-- {
		if !s.hour.match(hour) {
			continue
		}
		maxMinute := 59
		if onLimitDay && hour == limit.Hour() {
			maxMinute = limit.Minute()
		}
		for minute := maxMinute; minute >= 0; minute-- {
			if !s.minute.match(minute) {
				continue
			}
			candidate := day.Add(time.Duration(hour)*time.Hour + time.Duration(minute)*time.Minute)
			if !candidate.After(limit) {
				return candidate, true
			}
		}
	}
	return time.Time{}, false
}

func (s cronSpec) firstHitOfDay(day, limit time.Time, strictlyAfterDay bool) (time.Time, bool) {
	for hour := 0; hour <= 23; hour++ {
		if !s.hour.match(hour) {
			continue
		}
		for minute := 0; minute <= 59; minute++ {
			if !s.minute.match(minute) {
				continue
			}
			candidate := day.Add(time.Duration(hour)*time.Hour + time.Duration(minute)*time.Minute)
			if strictlyAfterDay || candidate.After(limit) {
				return candidate, true
			}
		}
	}
	return time.Time{}, false
}
