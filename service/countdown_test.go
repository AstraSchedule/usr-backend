package service

import (
	"AstraScheduleServerGo/model/dbTable"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScopeMatchesClass_ALL(t *testing.T) {
	assert.True(t, ScopeMatchesClass("ALL", "school/grade/class1"))
}

func TestScopeMatchesClass_School(t *testing.T) {
	assert.True(t, ScopeMatchesClass("school", "school/grade/class1"))
	assert.False(t, ScopeMatchesClass("other", "school/grade/class1"))
}

func TestScopeMatchesClass_Grade(t *testing.T) {
	assert.True(t, ScopeMatchesClass("school/grade", "school/grade/class1"))
	assert.False(t, ScopeMatchesClass("school/other", "school/grade/class1"))
}

func TestScopeMatchesClass_Class(t *testing.T) {
	assert.True(t, ScopeMatchesClass("school/grade/class1", "school/grade/class1"))
	assert.False(t, ScopeMatchesClass("school/grade/class2", "school/grade/class1"))
}

func TestScopeMatchesClass_EmptyScope(t *testing.T) {
	// Empty scope returns true (matches all)
	assert.True(t, ScopeMatchesClass("", "school/grade/class1"))
}

func TestFilterCountdownByScope_AllMatch(t *testing.T) {
	records := []dbTable.CountdownRecord{
		{ID: "1", Scope: []string{"ALL"}},
		{ID: "2", Scope: []string{"school"}},
	}
	result := FilterCountdownByScope(records, "school/grade/class1")
	assert.Equal(t, 2, len(result))
}

func TestFilterCountdownByScope_SomeMatch(t *testing.T) {
	records := []dbTable.CountdownRecord{
		{ID: "1", Scope: []string{"ALL"}},
		{ID: "2", Scope: []string{"other-school/grade/class1"}},
		{ID: "3", Scope: []string{"school/grade/class1"}},
	}
	result := FilterCountdownByScope(records, "school/grade/class1")
	assert.Equal(t, 2, len(result))
}

func TestFilterCountdownByScope_NoneMatch(t *testing.T) {
	records := []dbTable.CountdownRecord{
		{ID: "1", Scope: []string{"other"}},
		{ID: "2", Scope: []string{"another"}},
	}
	result := FilterCountdownByScope(records, "school/grade/class1")
	assert.Equal(t, 0, len(result))
}

func TestFilterCountdownByScope_EmptyRecords(t *testing.T) {
	result := FilterCountdownByScope([]dbTable.CountdownRecord{}, "school/grade/class1")
	assert.Equal(t, 0, len(result))
}
