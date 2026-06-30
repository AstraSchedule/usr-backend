package dbTable

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassList_UnmarshalJSON_OldFormat(t *testing.T) {
	input := `["物","数","语"]`
	var cl ClassList
	err := json.Unmarshal([]byte(input), &cl)
	require.NoError(t, err)
	assert.Equal(t, ClassList{{"物"}, {"数"}, {"语"}}, cl)
}

func TestClassList_UnmarshalJSON_NewFormat(t *testing.T) {
	input := `[["物","数"],["语"]]`
	var cl ClassList
	err := json.Unmarshal([]byte(input), &cl)
	require.NoError(t, err)
	assert.Equal(t, ClassList{{"物", "数"}, {"语"}}, cl)
}

func TestClassList_UnmarshalJSON_MixedFormat(t *testing.T) {
	input := `["物",["数","语"],"化"]`
	var cl ClassList
	err := json.Unmarshal([]byte(input), &cl)
	require.NoError(t, err)
	assert.Equal(t, ClassList{{"物"}, {"数", "语"}, {"化"}}, cl)
}

func TestClassList_UnmarshalJSON_EmptyArray(t *testing.T) {
	input := `[]`
	var cl ClassList
	err := json.Unmarshal([]byte(input), &cl)
	require.NoError(t, err)
	assert.Equal(t, ClassList{}, cl)
}

func TestClassList_MarshalJSON(t *testing.T) {
	cl := ClassList{{"物"}, {"数", "语"}}
	data, err := json.Marshal(cl)
	require.NoError(t, err)
	assert.Equal(t, `[["物"],["数","语"]]`, string(data))
}

func TestClassList_RoundTrip(t *testing.T) {
	original := ClassList{{"数"}, {"语", "英"}, {"物"}, {"化"}}
	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded ClassList
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, original, decoded)
}

func TestSchedule_JSON_Serialization(t *testing.T) {
	schedule := Schedule{
		ID:     1,
		School: "test-school",
		Grade:  "test-grade",
		Class:  "1班",
		DailyClasses: [7]DailyClass{
			{Timetable: "常日", ClassList: ClassList{{"数"}, {"语"}}},
			{},
			{},
			{},
			{},
			{},
			{},
		},
	}

	data, err := json.Marshal(schedule)
	require.NoError(t, err)

	var decoded Schedule
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "test-school", decoded.School)
	assert.Equal(t, "1班", decoded.Class)
	assert.Equal(t, "常日", decoded.DailyClasses[0].Timetable)
	assert.Equal(t, ClassList{{"数"}, {"语"}}, decoded.DailyClasses[0].ClassList)
}
