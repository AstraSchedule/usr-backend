package web

import (
	"AstraScheduleServerGo/model/dbTable"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSyncTimetableDividerKeys_NilDivider(t *testing.T) {
	cfg := &dbTable.TimetableConfig{
		Timetable: map[string]map[string]interface{}{
			"常日": {"早上1": 1},
			"午间": {"午1": 1},
		},
		Divider: nil,
	}
	syncTimetableDividerKeys(cfg)
	assert.Contains(t, cfg.Divider, "常日")
	assert.Contains(t, cfg.Divider, "午间")
}

func TestSyncTimetableDividerKeys_ExtraDividerKey(t *testing.T) {
	cfg := &dbTable.TimetableConfig{
		Timetable: map[string]map[string]interface{}{
			"常日": {"早上1": 1},
		},
		Divider: map[string][]int{
			"常日":  {3},
			"不存在": {5},
		},
	}
	syncTimetableDividerKeys(cfg)
	assert.Contains(t, cfg.Divider, "常日")
	assert.NotContains(t, cfg.Divider, "不存在")
}

func TestCloneStringMap_Nil(t *testing.T) {
	result := cloneStringMap(nil)
	assert.Equal(t, map[string]string{}, result)
}

func TestCloneStringMap(t *testing.T) {
	src := map[string]string{"a": "1", "b": "2"}
	result := cloneStringMap(src)
	assert.Equal(t, src, result)
	// Verify it's a copy
	result["c"] = "3"
	assert.NotContains(t, src, "c")
}

func TestCloneTimetableMap_Nil(t *testing.T) {
	result := cloneTimetableMap(nil)
	assert.Equal(t, map[string]map[string]interface{}{}, result)
}

func TestCloneTimetableMap(t *testing.T) {
	src := map[string]map[string]interface{}{
		"常日": {"k": "v"},
	}
	result := cloneTimetableMap(src)
	assert.Equal(t, "v", result["常日"]["k"])
	// Verify deep copy
	result["常日"]["k"] = "changed"
	assert.Equal(t, "v", src["常日"]["k"])
}

func TestCloneDividerMap_Nil(t *testing.T) {
	result := cloneDividerMap(nil)
	assert.Equal(t, map[string][]int{}, result)
}

func TestCloneDividerMap(t *testing.T) {
	src := map[string][]int{"常日": {1, 2, 3}}
	result := cloneDividerMap(src)
	assert.Equal(t, src["常日"], result["常日"])
	// Verify deep copy
	result["常日"][0] = 99
	assert.Equal(t, 1, src["常日"][0])
}

func TestCloneDailyClasses(t *testing.T) {
	src := [7]dbTable.DailyClass{
		{Timetable: "常日", ClassList: dbTable.ClassList{{"数"}}},
		{},
		{},
		{},
		{},
		{},
		{},
	}
	result := cloneDailyClasses(src)
	assert.Equal(t, "常日", result[0].Timetable)
	assert.Equal(t, dbTable.ClassList{{"数"}}, result[0].ClassList)
	// Note: cloneDailyClasses does shallow copy of inner slices
	// So modifying the inner slice affects both original and copy
	result[0].ClassList[0][0] = "语"
	assert.Equal(t, "语", src[0].ClassList[0][0]) // Shallow copy, both affected
}

func TestCloneDailyClasses_NilClassList(t *testing.T) {
	src := [7]dbTable.DailyClass{
		{Timetable: "常日", ClassList: nil},
	}
	result := cloneDailyClasses(src)
	assert.NotNil(t, result[0].ClassList)
	assert.Equal(t, 0, len(result[0].ClassList))
}
