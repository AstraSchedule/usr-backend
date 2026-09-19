package service

import (
	"AstraScheduleServerGo/model/dbTable"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// swapCondition 构造调课条目的生效条件：同一天用 date，跨天用覆盖两端的 range
func swapCondition(fromDate, toDate string) *dbTable.AutorunCondition {
	if fromDate == toDate {
		return &dbTable.AutorunCondition{Kind: dbTable.AutorunWhenDate, Date: fromDate}
	}
	start, end := fromDate, toDate
	if start > end {
		start, end = end, start
	}
	return &dbTable.AutorunCondition{Kind: dbTable.AutorunWhenRange, StartDate: start, EndDate: end}
}

// swapRecord 构造一条「调课」任务（v2 条目）
func swapRecord(fromDate string, fromPeriod int, toDate string, toPeriod int, level int) dbTable.AutorunRecord {
	return dbTable.AutorunRecord{
		HashID: "swap",
		EType:  dbTable.AutorunTypeLessonSwap,
		Scope:  []string{"ALL"},
		Level:  level,
		Entries: []dbTable.AutorunEntry{{
			ID:   "e1",
			When: swapCondition(fromDate, toDate),
			Action: map[string]interface{}{
				"swap": map[string]interface{}{
					"from": map[string]interface{}{"date": fromDate, "period": float64(fromPeriod)},
					"to":   map[string]interface{}{"date": toDate, "period": float64(toPeriod)},
				},
			},
		}},
	}
}

func resolveOn(records []dbTable.AutorunRecord, day time.Time) [7]dbTable.DailyClass {
	return ApplyScheduleRulesCtx(baseSchedule(), baseTimetable(), records, "s", "g", "c", RuleContext{Now: day, TermStart: "2025-10-13"})
}

// 同一天内交换两节：直接互换科目
func TestApplyScheduleRules_LessonSwapSameDay(t *testing.T) {
	records := []dbTable.AutorunRecord{swapRecord("2025-10-13", 1, "2025-10-13", 2, 1)}

	resolved := resolveOn(records, mondayDate())

	assert.Equal(t, dbTable.ClassList{{"语"}, {"数"}}, resolved[1].ClassList, "周一第 1/2 节应互换")

	// 其它日期不受影响
	other := resolveOn(records, time.Date(2025, 10, 14, 0, 0, 0, 0, time.Local))
	assert.Equal(t, dbTable.ClassList{{"英"}, {"课"}}, other[2].ClassList)
}

// 同一天内交换时，先按当前周数解析每周轮换，再互换
func TestApplyScheduleRules_LessonSwapSameDayWithRotation(t *testing.T) {
	base := baseSchedule()
	base[1].ClassList = dbTable.ClassList{{"数", "语"}, {"英"}}
	records := []dbTable.AutorunRecord{swapRecord("2025-10-20", 1, "2025-10-20", 2, 1)}

	// 2025-10-20 是第 2 周：第 1 节轮换到「语」
	resolved := ApplyScheduleRulesCtx(base, baseTimetable(), records, "s", "g", "c", RuleContext{Now: time.Date(2025, 10, 20, 0, 0, 0, 0, time.Local), TermStart: "2025-10-13"})

	assert.Equal(t, dbTable.ClassList{{"英"}, {"语"}}, resolved[1].ClassList,
		"应先按第 2 周解析出 语/英 再互换")
}

// 跨天互换：两端各自在自己那天生效，对方科目按对方日期所在周解析
func TestApplyScheduleRules_LessonSwapCrossDay(t *testing.T) {
	records := []dbTable.AutorunRecord{swapRecord("2025-10-13", 1, "2025-10-14", 1, 1)}

	monday := resolveOn(records, mondayDate())
	assert.Equal(t, dbTable.ClassList{{"英"}, {"语"}}, monday[1].ClassList, "周一第 1 节应拿到周二的科目")

	tuesday := resolveOn(records, time.Date(2025, 10, 14, 0, 0, 0, 0, time.Local))
	assert.Equal(t, dbTable.ClassList{{"数"}, {"课"}}, tuesday[2].ClassList, "周二第 1 节应拿到周一的科目")
}

// 跨天且跨周时，对方科目要按对方日期所在周解析轮换课表
func TestApplyScheduleRules_LessonSwapCrossWeek(t *testing.T) {
	base := baseSchedule()
	base[2].ClassList = dbTable.ClassList{{"体", "美"}}
	records := []dbTable.AutorunRecord{swapRecord("2025-10-13", 2, "2025-10-21", 1, 1)}

	// 2025-10-21 是第 2 周，周二第 1 节轮换为「美」
	monday := ApplyScheduleRulesCtx(base, baseTimetable(), records, "s", "g", "c", RuleContext{Now: mondayDate(), TermStart: "2025-10-13"})
	assert.Equal(t, dbTable.ClassList{{"数"}, {"美"}}, monday[1].ClassList)

	// 调课是 copy-on-write：解析结果不能污染调用方传入的 base 课表
	assert.Equal(t, dbTable.ClassList{{"数"}, {"语"}}, base[1].ClassList, "base 不应被改写")

	// 反过来：2025-10-21 那天应该拿到 2025-10-13 的周一第 2 节（「语」，与周数无关）
	tuesday := ApplyScheduleRulesCtx(base, baseTimetable(), records, "s", "g", "c", RuleContext{Now: time.Date(2025, 10, 21, 0, 0, 0, 0, time.Local), TermStart: "2025-10-13"})
	assert.Equal(t, dbTable.ClassList{{"语"}, {"课"}}, tuesday[2].ClassList)
}

// 调课与课程表调整同层：按用户设置的优先级 level 混排，最后生效的胜出
func TestApplyScheduleRules_LessonSwapAndScheduleShareLayer(t *testing.T) {
	scheduleRule := makeRecord(dbTable.AutorunTypeSchedule, []string{"ALL"}, 1, map[string]interface{}{
		"date": "2025-10-13",
		"schedule": map[string]interface{}{
			"periods": []interface{}{
				map[string]interface{}{"no": 1, "subject": "班会"},
				map[string]interface{}{"no": 2, "subject": "自习"},
			},
		},
	})

	// 调课优先级更高（level 2）→ 在整日替换之后再互换
	higher := resolveOn([]dbTable.AutorunRecord{scheduleRule, swapRecord("2025-10-13", 1, "2025-10-13", 2, 2)}, mondayDate())
	assert.Equal(t, dbTable.ClassList{{"自习"}, {"班会"}}, higher[1].ClassList)

	// 课程表调整优先级更高（level 2）→ 整日替换覆盖调课结果
	lower := resolveOn([]dbTable.AutorunRecord{swapRecord("2025-10-13", 1, "2025-10-13", 2, 1), makeRecord(dbTable.AutorunTypeSchedule, []string{"ALL"}, 2, map[string]interface{}{
		"date": "2025-10-13",
		"schedule": map[string]interface{}{
			"periods": []interface{}{
				map[string]interface{}{"no": 1, "subject": "班会"},
				map[string]interface{}{"no": 2, "subject": "自习"},
			},
		},
	})}, mondayDate())
	assert.Equal(t, dbTable.ClassList{{"班会"}, {"自习"}}, lower[1].ClassList)
}

// 条件不命中（只覆盖一端的日期）时，另一端那天不生效
func TestApplyScheduleRules_LessonSwapConditionGate(t *testing.T) {
	record := swapRecord("2025-10-13", 1, "2025-10-14", 1, 1)
	record.Entries[0].When = &dbTable.AutorunCondition{Kind: dbTable.AutorunWhenDate, Date: "2025-10-13"}

	tuesday := resolveOn([]dbTable.AutorunRecord{record}, time.Date(2025, 10, 14, 0, 0, 0, 0, time.Local))
	assert.Equal(t, dbTable.ClassList{{"英"}, {"课"}}, tuesday[2].ClassList, "条件未覆盖周二，周二应保持原样")
}

// 非法或越界的调课数据必须被安全忽略，且不影响其它条目
func TestApplyScheduleRules_LessonSwapInvalidData(t *testing.T) {
	invalid := []dbTable.AutorunRecord{
		{
			HashID: "bad-shape", EType: dbTable.AutorunTypeLessonSwap, Scope: []string{"ALL"}, Level: 1,
			Entries: []dbTable.AutorunEntry{{ID: "e1", Action: map[string]interface{}{"schedule": map[string]interface{}{}},
				When: &dbTable.AutorunCondition{Kind: dbTable.AutorunWhenDate, Date: "2025-10-13"}}},
		},
		{
			HashID: "bad-date", EType: dbTable.AutorunTypeLessonSwap, Scope: []string{"ALL"}, Level: 1,
			Entries: []dbTable.AutorunEntry{{ID: "e1", When: &dbTable.AutorunCondition{Kind: dbTable.AutorunWhenDate, Date: "2025-10-13"},
				Action: map[string]interface{}{"swap": map[string]interface{}{
					"from": map[string]interface{}{"date": "not-a-date", "period": 1},
					"to":   map[string]interface{}{"date": "2025-10-14", "period": 1},
				}}}},
		},
		{
			HashID: "bad-period", EType: dbTable.AutorunTypeLessonSwap, Scope: []string{"ALL"}, Level: 1,
			Entries: []dbTable.AutorunEntry{{ID: "e1", When: &dbTable.AutorunCondition{Kind: dbTable.AutorunWhenDate, Date: "2025-10-13"},
				Action: map[string]interface{}{"swap": map[string]interface{}{
					"from": map[string]interface{}{"date": "2025-10-13", "period": 0},
					"to":   map[string]interface{}{"date": "2025-10-13", "period": 9},
				}}}},
		},
		{
			HashID: "out-of-range", EType: dbTable.AutorunTypeLessonSwap, Scope: []string{"ALL"}, Level: 1,
			Entries: []dbTable.AutorunEntry{{ID: "e1", When: &dbTable.AutorunCondition{Kind: dbTable.AutorunWhenDate, Date: "2025-10-13"},
				Action: map[string]interface{}{"swap": map[string]interface{}{
					"from": map[string]interface{}{"date": "2025-10-13", "period": 1},
					"to":   map[string]interface{}{"date": "2025-10-13", "period": 7},
				}}}},
		},
	}

	resolved := resolveOn(invalid, mondayDate())
	assert.Equal(t, dbTable.ClassList{{"数"}, {"语"}}, resolved[1].ClassList)
}
