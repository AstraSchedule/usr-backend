package service

import (
	"AstraScheduleServerGo/model/dbTable"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 2026-09-01 是周二，所在周（周一 2026-08-31 起）为第 1 周
const testTermStart = "2026-09-01"

func day(y int, m time.Month, d int, hour int) time.Time {
	return time.Date(y, m, d, hour, 0, 0, 0, time.UTC)
}

func weeklyCondition(every, offset int) *dbTable.AutorunCondition {
	return &dbTable.AutorunCondition{Kind: dbTable.AutorunWhenWeekly, EveryWeeks: every, WeekOffset: offset}
}

func TestEntriesOf_LegacyParametersBecomesSingleDateEntry(t *testing.T) {
	record := dbTable.AutorunRecord{
		HashID:     "legacy",
		EType:      dbTable.AutorunTypeCompensation,
		Scope:      []string{"ALL"},
		Parameters: map[string]interface{}{"rule": map[string]interface{}{"date": "2026-09-01", "useDate": "2026-09-03"}},
	}

	entries := EntriesOf(record)
	require.Len(t, entries, 1)
	require.NotNil(t, entries[0].When)
	assert.Equal(t, dbTable.AutorunWhenDate, entries[0].When.Kind)
	assert.Equal(t, "2026-09-01", entries[0].When.Date)
	// date 被搬到条件上，内容里不再保留重复的 date
	assert.Equal(t, map[string]interface{}{"useDate": "2026-09-03"}, entries[0].Action)
	assert.False(t, entries[0].Disabled)
}

func TestEntriesOf_PrefersEntriesOverLegacyParameters(t *testing.T) {
	record := dbTable.AutorunRecord{
		HashID:     "v2",
		EType:      dbTable.AutorunTypeTimetable,
		Parameters: map[string]interface{}{"rule": map[string]interface{}{"date": "2026-09-01", "timetableId": "old"}},
		Entries: []dbTable.AutorunEntry{
			{ID: "e1", When: weeklyCondition(2, 0), Action: map[string]interface{}{"timetableId": "A"}},
			{ID: "e2", When: weeklyCondition(2, 1), Action: map[string]interface{}{"timetableId": "B"}},
		},
	}

	entries := EntriesOf(record)
	require.Len(t, entries, 2)
	assert.Equal(t, "A", entries[0].Action["timetableId"])
	assert.Equal(t, "B", entries[1].Action["timetableId"])
}

func TestEntriesOf_SkipsEntriesWithoutAction(t *testing.T) {
	record := dbTable.AutorunRecord{
		HashID:  "empty",
		EType:   dbTable.AutorunTypeSchedule,
		Entries: []dbTable.AutorunEntry{{ID: "e1", Action: map[string]interface{}{}}, {ID: "e2", Action: map[string]interface{}{"timetableId": "x"}}},
	}

	entries := EntriesOf(record)
	require.Len(t, entries, 1)
	assert.Equal(t, "e2", entries[0].ID)
}

func TestMatchCondition_Date(t *testing.T) {
	when := &dbTable.AutorunCondition{Kind: dbTable.AutorunWhenDate, Date: "2026-09-01"}
	assert.True(t, MatchCondition(when, RuleContext{Now: day(2026, time.September, 1, 10)}))
	assert.False(t, MatchCondition(when, RuleContext{Now: day(2026, time.September, 2, 10)}))
	// 单日条件与学期起始日无关
	assert.True(t, MatchCondition(when, RuleContext{Now: day(2026, time.September, 1, 10), TermStart: testTermStart}))
}

func TestMatchCondition_Range(t *testing.T) {
	when := &dbTable.AutorunCondition{Kind: dbTable.AutorunWhenRange, StartDate: "2026-09-07", EndDate: "2026-09-11"}
	assert.False(t, MatchCondition(when, RuleContext{Now: day(2026, time.September, 6, 23)}))
	assert.True(t, MatchCondition(when, RuleContext{Now: day(2026, time.September, 7, 0)}))
	assert.True(t, MatchCondition(when, RuleContext{Now: day(2026, time.September, 11, 23)}))
	assert.False(t, MatchCondition(when, RuleContext{Now: day(2026, time.September, 12, 0)}))
}

func TestMatchCondition_RangeWithWeekdays(t *testing.T) {
	when := &dbTable.AutorunCondition{Kind: dbTable.AutorunWhenRange, StartDate: "2026-09-01", EndDate: "2026-09-30", Weekdays: []int{1, 3}}
	// 2026-09-02 周三命中，09-03 周四不命中
	assert.True(t, MatchCondition(when, RuleContext{Now: day(2026, time.September, 2, 8)}))
	assert.False(t, MatchCondition(when, RuleContext{Now: day(2026, time.September, 3, 8)}))
}

func TestMatchCondition_Weekly(t *testing.T) {
	ctx := func(d time.Time) RuleContext {
		return RuleContext{Now: d, TermStart: testTermStart}
	}
	// 第 1、3、5… 周
	odd := weeklyCondition(2, 0)
	assert.True(t, MatchCondition(odd, ctx(day(2026, time.September, 1, 8))), "第 1 周")
	assert.False(t, MatchCondition(odd, ctx(day(2026, time.September, 8, 8))), "第 2 周")
	assert.True(t, MatchCondition(odd, ctx(day(2026, time.September, 15, 8))), "第 3 周")

	// 第 2、4、6… 周
	even := weeklyCondition(2, 1)
	assert.False(t, MatchCondition(even, ctx(day(2026, time.September, 1, 8))))
	assert.True(t, MatchCondition(even, ctx(day(2026, time.September, 8, 8))))
}

func TestMatchCondition_WeeklyFourWeekRotation(t *testing.T) {
	// 四周轮换：偏移 0/1/2/3 分别在第 1/2/3/4 周命中，第 5 周回到偏移 0
	dates := []time.Time{
		day(2026, time.September, 1, 8),  // 第 1 周
		day(2026, time.September, 8, 8),  // 第 2 周
		day(2026, time.September, 15, 8), // 第 3 周
		day(2026, time.September, 22, 8), // 第 4 周
	}
	for offset := 0; offset < 4; offset++ {
		when := weeklyCondition(4, offset)
		for i, d := range dates {
			expected := i == offset
			got := MatchCondition(when, RuleContext{Now: d, TermStart: testTermStart})
			assert.Equal(t, expected, got, "offset=%d week=%d", offset, i+1)
		}
		// 第 5 周（09-29）回到偏移 0
		got := MatchCondition(when, RuleContext{Now: day(2026, time.September, 29, 8), TermStart: testTermStart})
		assert.Equal(t, offset == 0, got, "第 5 周应回到偏移 0")
	}
}

func TestMatchCondition_WeeklyWithWeekdayFilter(t *testing.T) {
	when := weeklyCondition(2, 0)
	when.Weekdays = []int{3} // 周三
	assert.True(t, MatchCondition(when, RuleContext{Now: day(2026, time.September, 2, 8), TermStart: testTermStart}), "第 1 周周三")
	assert.False(t, MatchCondition(when, RuleContext{Now: day(2026, time.September, 3, 8), TermStart: testTermStart}), "第 1 周周四")
	assert.False(t, MatchCondition(when, RuleContext{Now: day(2026, time.September, 9, 8), TermStart: testTermStart}), "第 2 周周三")
}

func TestMatchCondition_WeeklyWithoutTermStartFallsBackToFirstWeek(t *testing.T) {
	// 没有学期起始日时退化为「第 1 周」语义：偏移 0 命中，偏移 1 不命中
	assert.True(t, MatchCondition(weeklyCondition(2, 0), RuleContext{Now: day(2026, time.September, 1, 8)}))
	assert.False(t, MatchCondition(weeklyCondition(2, 1), RuleContext{Now: day(2026, time.September, 1, 8)}))
}

func TestMatchCondition_EventMatchesDayLevel(t *testing.T) {
	when := &dbTable.AutorunCondition{Kind: dbTable.AutorunWhenEvent, Event: dbTable.AutorunEventClassStart, Period: 3}
	assert.True(t, MatchCondition(when, RuleContext{Now: day(2026, time.September, 1, 8)}))

	when.Weekdays = []int{1}
	assert.True(t, MatchCondition(when, RuleContext{Now: day(2026, time.September, 7, 8)}), "周一")
	assert.False(t, MatchCondition(when, RuleContext{Now: day(2026, time.September, 8, 8)}), "周二")
}

func TestMatchCondition_CronWithDuration(t *testing.T) {
	when := &dbTable.AutorunCondition{Kind: dbTable.AutorunWhenCron, Cron: "0 8 * * *", Duration: 60}
	assert.False(t, MatchCondition(when, RuleContext{Now: day(2026, time.September, 1, 7)}))
	assert.True(t, MatchCondition(when, RuleContext{Now: day(2026, time.September, 1, 8)}))
	assert.True(t, MatchCondition(when, RuleContext{Now: time.Date(2026, time.September, 1, 8, 59, 0, 0, time.UTC)}))
	assert.False(t, MatchCondition(when, RuleContext{Now: time.Date(2026, time.September, 1, 9, 1, 0, 0, time.UTC)}))
}

func TestMatchCondition_CronUntilNextHit(t *testing.T) {
	when := &dbTable.AutorunCondition{Kind: dbTable.AutorunWhenCron, Cron: "0 8 * * *"}
	// 未填 duration：从命中时刻持续到下一次命中
	assert.True(t, MatchCondition(when, RuleContext{Now: time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)}))
	assert.True(t, MatchCondition(when, RuleContext{Now: time.Date(2026, time.September, 2, 7, 0, 0, 0, time.UTC)}))
}

func TestMatchCondition_InvalidAndUnknownKinds(t *testing.T) {
	assert.False(t, MatchCondition(&dbTable.AutorunCondition{Kind: "unknown"}, RuleContext{Now: day(2026, time.September, 1, 8)}))
	assert.False(t, MatchCondition(&dbTable.AutorunCondition{Kind: dbTable.AutorunWhenDate, Date: "not-a-date"}, RuleContext{Now: day(2026, time.September, 1, 8)}))
	assert.False(t, MatchCondition(&dbTable.AutorunCondition{Kind: dbTable.AutorunWhenCron, Cron: "bad"}, RuleContext{Now: day(2026, time.September, 1, 8)}))
	assert.True(t, MatchCondition(nil, RuleContext{Now: day(2026, time.September, 1, 8)}))
}

func TestMatchEntry_Disabled(t *testing.T) {
	entry := dbTable.AutorunEntry{Disabled: true, Action: map[string]interface{}{"timetableId": "A"}}
	assert.False(t, MatchEntry(entry, RuleContext{Now: day(2026, time.September, 1, 8)}))
}

func TestTaskStatus_LegacySingleDateUnchanged(t *testing.T) {
	record := dbTable.AutorunRecord{
		EType:      dbTable.AutorunTypeTimetable,
		Parameters: map[string]interface{}{"rule": map[string]interface{}{"date": "2026-09-01", "timetableId": "A"}},
	}
	assert.Equal(t, 0, TaskStatus(record, day(2026, time.August, 31, 12)))
	assert.Equal(t, 1, TaskStatus(record, day(2026, time.September, 1, 12)))
	assert.Equal(t, 2, TaskStatus(record, day(2026, time.September, 2, 12)))
}

func TestTaskStatus_RangeAndOpenEnded(t *testing.T) {
	ranged := dbTable.AutorunRecord{
		EType:   dbTable.AutorunTypeTimetable,
		Entries: []dbTable.AutorunEntry{{ID: "e1", When: &dbTable.AutorunCondition{Kind: dbTable.AutorunWhenRange, StartDate: "2026-09-07", EndDate: "2026-09-11"}, Action: map[string]interface{}{"timetableId": "exam"}}},
	}
	assert.Equal(t, 0, TaskStatus(ranged, day(2026, time.September, 6, 12)))
	assert.Equal(t, 1, TaskStatus(ranged, day(2026, time.September, 9, 12)))
	assert.Equal(t, 2, TaskStatus(ranged, day(2026, time.September, 12, 12)))

	// 每周轮换没有终点：长期生效中
	recurring := dbTable.AutorunRecord{
		EType:   dbTable.AutorunTypeTimetable,
		Entries: []dbTable.AutorunEntry{{ID: "e1", When: weeklyCondition(2, 0), Action: map[string]interface{}{"timetableId": "A"}}},
	}
	assert.Equal(t, 1, TaskStatus(recurring, day(2026, time.September, 8, 12)))
	assert.Equal(t, 1, TaskStatus(recurring, day(2027, time.December, 31, 12)))
}

func TestTaskStatus_EmptyEntries(t *testing.T) {
	record := dbTable.AutorunRecord{EType: dbTable.AutorunTypeTimetable}
	assert.Equal(t, 0, TaskStatus(record, day(2026, time.September, 1, 12)))
}

func TestTaskWindow_UnionKeepsUnboundedSide(t *testing.T) {
	// 一条单日条目 + 一条无终点的每周轮换条目：单日条目结束后任务整体仍应「生效中」
	record := dbTable.AutorunRecord{
		EType: dbTable.AutorunTypeTimetable,
		Entries: []dbTable.AutorunEntry{
			{ID: "e1", When: &dbTable.AutorunCondition{Kind: dbTable.AutorunWhenDate, Date: "2026-09-01"}, Action: map[string]interface{}{"timetableId": "exam"}},
			{ID: "e2", When: weeklyCondition(2, 0), Action: map[string]interface{}{"timetableId": "A"}},
		},
	}
	assert.Equal(t, 1, TaskStatus(record, day(2026, time.September, 1, 12)))
	assert.Equal(t, 1, TaskStatus(record, day(2026, time.October, 1, 12)), "无终点条目的并集不应被判为已过期")
}

func TestTaskStatus_DisabledEntriesIgnored(t *testing.T) {
	record := dbTable.AutorunRecord{
		EType: dbTable.AutorunTypeTimetable,
		Entries: []dbTable.AutorunEntry{
			{ID: "e1", Disabled: true, When: &dbTable.AutorunCondition{Kind: dbTable.AutorunWhenDate, Date: "2026-09-01"}, Action: map[string]interface{}{"timetableId": "exam"}},
		},
	}
	// 全部条目停用：任务等同于空任务，不应该被停用条目拖成「生效中」
	assert.Equal(t, 0, TaskStatus(record, day(2026, time.September, 1, 12)))

	withEnabled := record
	withEnabled.Entries = append(withEnabled.Entries, dbTable.AutorunEntry{ID: "e2", When: &dbTable.AutorunCondition{Kind: dbTable.AutorunWhenDate, Date: "2026-09-05"}, Action: map[string]interface{}{"timetableId": "A"}})
	assert.Equal(t, 0, TaskStatus(withEnabled, day(2026, time.September, 1, 12)), "只有尚未开始的启用条目")
	assert.Equal(t, 1, TaskStatus(withEnabled, day(2026, time.September, 5, 12)))
	assert.Equal(t, 2, TaskStatus(withEnabled, day(2026, time.September, 6, 12)))
}

func TestDynamicVersionBucket(t *testing.T) {
	now := day(2026, time.September, 1, 10)
	weekOnly := dbTable.AutorunRecord{
		EType: dbTable.AutorunTypeTimetable, Scope: []string{"ALL"},
		Entries: []dbTable.AutorunEntry{{ID: "e1", When: weeklyCondition(2, 0), Action: map[string]interface{}{"timetableId": "A"}}},
	}
	// 纯周次条件在周内不会变化，版本不需要时间桶（保持 304 缓存）
	assert.Equal(t, "", DynamicVersionBucket([]dbTable.AutorunRecord{weekOnly}, "s", "g", "c", now))

	dated := dbTable.AutorunRecord{
		EType: dbTable.AutorunTypeTimetable, Scope: []string{"s"},
		Entries: []dbTable.AutorunEntry{{ID: "e1", When: &dbTable.AutorunCondition{Kind: dbTable.AutorunWhenDate, Date: "2026-09-01"}, Action: map[string]interface{}{"timetableId": "A"}}},
	}
	bucket := DynamicVersionBucket([]dbTable.AutorunRecord{dated}, "s", "g", "c", now)
	assert.NotEqual(t, "", bucket)
	// 同一个 5 分钟桶内保持稳定，跨桶变化
	assert.Equal(t, bucket, DynamicVersionBucket([]dbTable.AutorunRecord{dated}, "s", "g", "c", now.Add(2*time.Minute)))
	assert.NotEqual(t, bucket, DynamicVersionBucket([]dbTable.AutorunRecord{dated}, "s", "g", "c", now.Add(6*time.Minute)))

	// 作用域不匹配 / 任务停用 / 客户端配置类型都不产生时间桶
	assert.Equal(t, "", DynamicVersionBucket([]dbTable.AutorunRecord{dated}, "other", "g", "c", now))
	disabled := dated
	disabled.Disabled = true
	assert.Equal(t, "", DynamicVersionBucket([]dbTable.AutorunRecord{disabled}, "s", "g", "c", now))
	clientConfig := dated
	clientConfig.EType = dbTable.AutorunTypeClientConfig
	assert.Equal(t, "", DynamicVersionBucket([]dbTable.AutorunRecord{clientConfig}, "s", "g", "c", now))
}

func TestCollectClientConfigRules_FiltersByScopeAndType(t *testing.T) {
	settings := map[string]interface{}{"isWindowAlwaysOnTop": true}
	records := []dbTable.AutorunRecord{
		{
			HashID: "cfg-class", EType: dbTable.AutorunTypeClientConfig, Scope: []string{"s/g/c"}, Level: 5,
			Entries: []dbTable.AutorunEntry{{ID: "e1", When: &dbTable.AutorunCondition{Kind: dbTable.AutorunWhenEvent, Event: dbTable.AutorunEventClassStart, Period: 1}, Action: map[string]interface{}{"settings": settings}}},
		},
		{
			HashID: "cfg-other", EType: dbTable.AutorunTypeClientConfig, Scope: []string{"s/g/other"},
			Entries: []dbTable.AutorunEntry{{ID: "e1", Action: map[string]interface{}{"settings": settings}}},
		},
		{
			HashID: "cfg-disabled", EType: dbTable.AutorunTypeClientConfig, Scope: []string{"ALL"}, Disabled: true,
			Entries: []dbTable.AutorunEntry{{ID: "e1", Action: map[string]interface{}{"settings": settings}}},
		},
		{
			HashID: "not-config", EType: dbTable.AutorunTypeTimetable, Scope: []string{"ALL"},
			Entries: []dbTable.AutorunEntry{{ID: "e1", Action: map[string]interface{}{"timetableId": "A"}}},
		},
	}

	rules := CollectClientConfigRules(records, "s", "g", "c")
	require.Len(t, rules, 1)
	assert.Equal(t, "cfg-class", rules[0].TaskID)
	assert.Equal(t, 5, rules[0].Priority)
	assert.Equal(t, 3, rules[0].Specificity)
	assert.Equal(t, settings, rules[0].Settings)
}

func TestApplyScheduleRulesCtx_WeeklyTimetableRotation(t *testing.T) {
	// Issue #57 核心场景：每两周轮换作息表，一条任务两条条目
	timetable := baseTimetable()
	timetable["暑期"] = map[string]interface{}{"09:00-10:00": 0}
	record := dbTable.AutorunRecord{
		HashID: "rotation", EType: dbTable.AutorunTypeTimetable, Scope: []string{"ALL"},
		Entries: []dbTable.AutorunEntry{
			{ID: "e1", When: weeklyCondition(2, 0), Action: map[string]interface{}{"timetableId": "exam"}},
			{ID: "e2", When: weeklyCondition(2, 1), Action: map[string]interface{}{"timetableId": "暑期"}},
		},
	}
	records := []dbTable.AutorunRecord{record}

	week1 := ApplyScheduleRulesCtx(baseSchedule(), timetable, records, "s", "g", "c",
		RuleContext{Now: day(2026, time.September, 1, 8), TermStart: testTermStart})
	assert.Equal(t, "exam", week1[2].Timetable, "第 1 周使用 exam 作息")

	week2 := ApplyScheduleRulesCtx(baseSchedule(), timetable, records, "s", "g", "c",
		RuleContext{Now: day(2026, time.September, 8, 8), TermStart: testTermStart})
	assert.Equal(t, "暑期", week2[2].Timetable, "第 2 周使用暑期作息")

	week3 := ApplyScheduleRulesCtx(baseSchedule(), timetable, records, "s", "g", "c",
		RuleContext{Now: day(2026, time.September, 15, 8), TermStart: testTermStart})
	assert.Equal(t, "exam", week3[2].Timetable, "第 3 周回到 exam 作息")
}

func TestApplyScheduleRulesCtx_DateRangeReplacesTimetable(t *testing.T) {
	// 一段时间内整体更换作息表
	record := dbTable.AutorunRecord{
		HashID: "exam-week", EType: dbTable.AutorunTypeTimetable, Scope: []string{"ALL"},
		Entries: []dbTable.AutorunEntry{{ID: "e1", When: &dbTable.AutorunCondition{Kind: dbTable.AutorunWhenRange, StartDate: "2026-09-07", EndDate: "2026-09-11"}, Action: map[string]interface{}{"timetableId": "exam"}}},
	}

	inside := ApplyScheduleRulesCtx(baseSchedule(), baseTimetable(), []dbTable.AutorunRecord{record}, "s", "g", "c",
		RuleContext{Now: day(2026, time.September, 9, 8), TermStart: testTermStart})
	assert.Equal(t, "exam", inside[3].Timetable, "范围内使用 exam 作息")

	outside := ApplyScheduleRulesCtx(baseSchedule(), baseTimetable(), []dbTable.AutorunRecord{record}, "s", "g", "c",
		RuleContext{Now: day(2026, time.September, 14, 8), TermStart: testTermStart})
	assert.Equal(t, "常日", outside[1].Timetable, "范围外恢复常日作息")
}

func TestApplyScheduleRulesCtx_SkipsDisabledTaskAndEntries(t *testing.T) {
	disabledTask := dbTable.AutorunRecord{
		HashID: "off", EType: dbTable.AutorunTypeTimetable, Scope: []string{"ALL"}, Disabled: true,
		Entries: []dbTable.AutorunEntry{{ID: "e1", When: &dbTable.AutorunCondition{Kind: dbTable.AutorunWhenDate, Date: "2026-09-01"}, Action: map[string]interface{}{"timetableId": "exam"}}},
	}
	disabledEntry := dbTable.AutorunRecord{
		HashID: "half", EType: dbTable.AutorunTypeTimetable, Scope: []string{"ALL"},
		Entries: []dbTable.AutorunEntry{
			{ID: "e1", Disabled: true, When: &dbTable.AutorunCondition{Kind: dbTable.AutorunWhenDate, Date: "2026-09-01"}, Action: map[string]interface{}{"timetableId": "exam"}},
			{ID: "e2", When: &dbTable.AutorunCondition{Kind: dbTable.AutorunWhenDate, Date: "2026-09-01"}, Action: map[string]interface{}{"timetableId": "常日"}},
		},
	}

	ctx := RuleContext{Now: day(2026, time.September, 1, 8), TermStart: testTermStart}
	resolved := ApplyScheduleRulesCtx(baseSchedule(), baseTimetable(), []dbTable.AutorunRecord{disabledTask, disabledEntry}, "s", "g", "c", ctx)
	assert.Equal(t, "常日", resolved[2].Timetable, "停用任务被跳过，停用条目被跳过，仅启用条目生效")
}
