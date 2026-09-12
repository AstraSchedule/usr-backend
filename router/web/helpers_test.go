package web

import (
	"AstraScheduleServerGo/model/dbTable"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseScopeInput_Nil(t *testing.T) {
	result := parseScopeInput(nil)
	assert.Equal(t, []string{"ALL"}, result)
}

func TestParseScopeInput_String(t *testing.T) {
	result := parseScopeInput("school/grade/class")
	assert.Equal(t, []string{"school/grade/class"}, result)
}

func TestParseScopeInput_StringArray(t *testing.T) {
	input := []interface{}{"scope1", "scope2"}
	result := parseScopeInput(input)
	assert.Equal(t, []string{"scope1", "scope2"}, result)
}

func TestParseScopeInput_GoStringArray(t *testing.T) {
	input := []string{"scope1", "scope2"}
	result := parseScopeInput(input)
	assert.Equal(t, []string{"scope1", "scope2"}, result)
}

func TestToString_Int(t *testing.T) {
	assert.Equal(t, "42", toString(42))
}

func TestToString_Float(t *testing.T) {
	assert.Equal(t, "3.14", toString(3.14))
}

func TestToString_String(t *testing.T) {
	assert.Equal(t, "hello", toString("hello"))
}

func TestToString_Bool(t *testing.T) {
	assert.Equal(t, "true", toString(true))
}

func TestToString_Nil(t *testing.T) {
	assert.Equal(t, "", toString(nil))
}

func TestParseScope_FullScope(t *testing.T) {
	school, grade, class, ok := parseScope("school1/grade1/class1")
	assert.True(t, ok)
	assert.Equal(t, "school1", school)
	assert.Equal(t, "grade1", grade)
	assert.Equal(t, "class1", class)
}

func TestParseScope_GradeScope(t *testing.T) {
	// parseScope requires exactly 3 parts, so grade-only scope returns false
	school, grade, class, ok := parseScope("school1/grade1")
	assert.False(t, ok)
	assert.Equal(t, "", school)
	assert.Equal(t, "", grade)
	assert.Equal(t, "", class)
}

func TestParseScope_SchoolScope(t *testing.T) {
	// parseScope requires exactly 3 parts, so school-only scope returns false
	school, grade, class, ok := parseScope("school1")
	assert.False(t, ok)
	assert.Equal(t, "", school)
	assert.Equal(t, "", grade)
	assert.Equal(t, "", class)
}

func TestParseScope_Empty(t *testing.T) {
	_, _, _, ok := parseScope("")
	assert.False(t, ok)
}

func TestParseClassList(t *testing.T) {
	input := dbTable.ClassList{{"数"}, {"语"}, {"英"}}
	result := parseClassList(input)
	assert.Equal(t, dbTable.ClassList{{"数"}, {"语"}, {"英"}}, result)
}

func TestMakeHashID(t *testing.T) {
	scope := []string{"ALL"}
	params := map[string]interface{}{
		"date": "2025-10-15",
	}
	hash1 := makeHashID(dbTable.AutorunTypeSchedule, scope, 1, params)
	hash2 := makeHashID(dbTable.AutorunTypeSchedule, scope, 1, params)
	assert.Equal(t, hash1, hash2)
	assert.NotEmpty(t, hash1)
}

func TestMakeHashID_DifferentInputs(t *testing.T) {
	scope1 := []string{"ALL"}
	scope2 := []string{"school"}
	params := map[string]interface{}{}

	hash1 := makeHashID(2, scope1, 1, params)
	hash2 := makeHashID(2, scope2, 1, params)
	assert.NotEqual(t, hash1, hash2)
}

func taskEntries(timetableID string) []dbTable.AutorunEntry {
	return []dbTable.AutorunEntry{{
		ID:     "e1",
		When:   &dbTable.AutorunCondition{Kind: dbTable.AutorunWhenDate, Date: "2026-09-01"},
		Action: map[string]interface{}{"timetableId": timetableID},
	}}
}

func TestMakeTaskHashID_StableForSameInput(t *testing.T) {
	entries := taskEntries("exam")
	hash1 := makeTaskHashID(dbTable.AutorunTypeTimetable, []string{"s", "g"}, 3, "轮换作息", entries)
	hash2 := makeTaskHashID(dbTable.AutorunTypeTimetable, []string{"s", "g"}, 3, "轮换作息", entries)
	assert.Equal(t, hash1, hash2)
	assert.Len(t, hash1, 16)
}

func TestMakeTaskHashID_ScopeOrderIrrelevant(t *testing.T) {
	entries := taskEntries("exam")
	hash1 := makeTaskHashID(dbTable.AutorunTypeTimetable, []string{"s", "g", "c"}, 1, "n", entries)
	hash2 := makeTaskHashID(dbTable.AutorunTypeTimetable, []string{"c", "s", "g"}, 1, "n", entries)
	assert.Equal(t, hash1, hash2, "作用域顺序不应改变任务 ID")
}

func TestMakeTaskHashID_DistinguishesFieldBoundaries(t *testing.T) {
	// 手工拼接哈希时 "a=b|c=d" 与 "a=b|c=d" 这类跨字段的相同字节串会误判为同一条任务，
	// 改用 JSON 编码后字段边界明确，以下两两都必须不同
	first := makeTaskHashID(dbTable.AutorunTypeTimetable, []string{"s"}, 1, "name",
		[]dbTable.AutorunEntry{{ID: "e1", Action: map[string]interface{}{"a": "x|b=y"}}})
	second := makeTaskHashID(dbTable.AutorunTypeTimetable, []string{"s"}, 1, "name",
		[]dbTable.AutorunEntry{{ID: "e1", Action: map[string]interface{}{"a": "x", "b": "y"}}})
	assert.NotEqual(t, first, second)

	// 分隔符出现在值里不应与其他字段混淆
	third := makeTaskHashID(dbTable.AutorunTypeTimetable, []string{"s"}, 1, "name",
		[]dbTable.AutorunEntry{{ID: "e1", Action: map[string]interface{}{"timetableId": "a~b"}}})
	fourth := makeTaskHashID(dbTable.AutorunTypeTimetable, []string{"s"}, 1, "name",
		[]dbTable.AutorunEntry{{ID: "e1", Action: map[string]interface{}{"timetableId": "a"}, Disabled: true}})
	assert.NotEqual(t, third, fourth)

	// 类型 / 优先级 / 名称 / 条目条件都要参与哈希
	base := makeTaskHashID(dbTable.AutorunTypeTimetable, []string{"s"}, 1, "name", taskEntries("exam"))
	assert.NotEqual(t, base, makeTaskHashID(dbTable.AutorunTypeAll, []string{"s"}, 1, "name", taskEntries("exam")))
	assert.NotEqual(t, base, makeTaskHashID(dbTable.AutorunTypeTimetable, []string{"s"}, 2, "name", taskEntries("exam")))
	assert.NotEqual(t, base, makeTaskHashID(dbTable.AutorunTypeTimetable, []string{"s"}, 1, "other", taskEntries("exam")))
	assert.NotEqual(t, base, makeTaskHashID(dbTable.AutorunTypeTimetable, []string{"s"}, 1, "name", taskEntries("常日")))

	weekCondition := taskEntries("exam")
	weekCondition[0].When = &dbTable.AutorunCondition{Kind: dbTable.AutorunWhenWeekly, EveryWeeks: 2, WeekOffset: 1}
	assert.NotEqual(t, base, makeTaskHashID(dbTable.AutorunTypeTimetable, []string{"s"}, 1, "name", weekCondition))
}

func TestMakeTaskHashID_PinnedRepresentativeTask(t *testing.T) {
	// 钉住一个代表性任务的当前 ID：编码策略若被意外修改，这里会立刻失败
	got := makeTaskHashID(dbTable.AutorunTypeTimetable, []string{"ALL"}, 5, "轮换作息",
		[]dbTable.AutorunEntry{
			{ID: "e1", When: &dbTable.AutorunCondition{Kind: dbTable.AutorunWhenWeekly, EveryWeeks: 2, WeekOffset: 0}, Action: map[string]interface{}{"timetableId": "exam"}},
			{ID: "e2", When: &dbTable.AutorunCondition{Kind: dbTable.AutorunWhenWeekly, EveryWeeks: 2, WeekOffset: 1}, Action: map[string]interface{}{"timetableId": "常日"}},
		})
	assert.Equal(t, "021eaf4c67784700", got)
}

func TestStringsFromScope(t *testing.T) {
	scope := []string{"school1", "grade1", "class1"}
	result := stringsFromScope(scope)
	assert.Contains(t, result, "school1")
	assert.Contains(t, result, "grade1")
	assert.Contains(t, result, "class1")
}

func TestStableMapString(t *testing.T) {
	m := map[string]interface{}{
		"b": 2,
		"a": 1,
	}
	result1 := stableMapString(m)
	result2 := stableMapString(m)
	assert.Equal(t, result1, result2)
	assert.NotEmpty(t, result1)
}

func TestServiceAsInt(t *testing.T) {
	val, ok := serviceAsInt(42)
	assert.True(t, ok)
	assert.Equal(t, 42, val)

	val, ok = serviceAsInt("123")
	assert.True(t, ok)
	assert.Equal(t, 123, val)

	val, ok = serviceAsInt("abc")
	assert.False(t, ok)

	val, ok = serviceAsInt(nil)
	assert.False(t, ok)
}

func TestMergeScopes_UnionDedupe(t *testing.T) {
	got := mergeScopes([]string{"s1/g1", "ALL"}, []string{"ALL", "s2/g2", " s1/g1 "})
	assert.Equal(t, []string{"s1/g1", "ALL", "s2/g2"}, got)
}

func TestMergeScopes_EmptyOld(t *testing.T) {
	assert.Equal(t, []string{"s1"}, mergeScopes(nil, []string{"s1"}))
	assert.Empty(t, mergeScopes(nil, nil))
}
