package web

import (
	"net/http"
	"testing"

	"AstraScheduleServerGo/model/dbTable"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// swapEntry 构造一条调课条目：默认条件覆盖两端日期
func swapEntry(fromDate string, fromPeriod interface{}, toDate string, toPeriod interface{}) map[string]interface{} {
	return map[string]interface{}{
		"when": map[string]interface{}{
			"kind": "range", "startDate": "2026-09-14", "endDate": "2026-09-15",
		},
		"action": map[string]interface{}{
			"swap": map[string]interface{}{
				"from": map[string]interface{}{"date": fromDate, "period": fromPeriod},
				"to":   map[string]interface{}{"date": toDate, "period": toPeriod},
			},
		},
	}
}

func swapTaskBody(entry map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"name": "调课", "type": dbTable.AutorunTypeLessonSwap, "scope": []string{"ALL"},
		"priority": 3, "entries": []map[string]interface{}{entry},
	}
}

func TestPutAutorunTask_LessonSwap(t *testing.T) {
	ensureTestDB()
	router := taskRouter(t)

	w := doRequest(t, router, "PUT", "/web/autorun/task", swapTaskBody(swapEntry("2026-09-14", 3, "2026-09-15", 1)))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	item := fetchAutorunDetail(t, router, w)
	assert.Equal(t, "LESSON_SWAP", item["type"], "类型应映射为 LESSON_SWAP")
	assert.Equal(t, float64(3), item["priority"])

	swap, ok := autorunContent(t, item)["swap"].(map[string]interface{})
	require.True(t, ok, "content.swap 应为对象")
	from, ok := swap["from"].(map[string]interface{})
	require.True(t, ok, "content.swap.from 应为对象")
	to, ok := swap["to"].(map[string]interface{})
	require.True(t, ok, "content.swap.to 应为对象")
	assert.Equal(t, "2026-09-14", from["date"])
	assert.Equal(t, float64(3), from["period"])
	assert.Equal(t, "2026-09-15", to["date"])
	assert.Equal(t, float64(1), to["period"])
}

func TestPutAutorunTask_LessonSwapSameDay(t *testing.T) {
	ensureTestDB()
	router := taskRouter(t)

	entry := swapEntry("2026-09-14", 1, "2026-09-14", 2)
	entry["when"] = map[string]interface{}{"kind": "date", "date": "2026-09-14"}
	w := doRequest(t, router, "PUT", "/web/autorun/task", swapTaskBody(entry))

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

func TestPutAutorunTask_LessonSwapInvalid(t *testing.T) {
	ensureTestDB()
	router := taskRouter(t)

	validFrom := map[string]interface{}{"date": "2026-09-14", "period": 3}
	validTo := map[string]interface{}{"date": "2026-09-15", "period": 1}
	cases := []struct {
		name   string
		action map[string]interface{}
		detail string
	}{
		{
			"缺少 swap",
			map[string]interface{}{"schedule": map[string]interface{}{}},
			"swap 必须为对象",
		},
		{
			"from 不是对象",
			map[string]interface{}{"swap": map[string]interface{}{"from": "2026-09-14", "to": validTo}},
			"swap.from 必须为对象",
		},
		{
			"from 日期非法",
			map[string]interface{}{"swap": map[string]interface{}{
				"from": map[string]interface{}{"date": "2026-13-01", "period": 3}, "to": validTo}},
			"swap.from.date 格式错误",
		},
		{
			"to 缺少节次",
			map[string]interface{}{"swap": map[string]interface{}{
				"from": validFrom, "to": map[string]interface{}{"date": "2026-09-15"}}},
			"swap.to.period 必须为正整数",
		},
		{
			"节次为零",
			map[string]interface{}{"swap": map[string]interface{}{
				"from": map[string]interface{}{"date": "2026-09-14", "period": 0}, "to": validTo}},
			"swap.from.period 必须为正整数",
		},
		{
			"两端为同一节课",
			map[string]interface{}{"swap": map[string]interface{}{
				"from": validFrom, "to": map[string]interface{}{"date": "2026-09-14", "period": 3}}},
			"swap.from 与 swap.to 不能是同一节课",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry := map[string]interface{}{"action": tc.action}
			w := doRequest(t, router, "PUT", "/web/autorun/task", swapTaskBody(entry))
			require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
			assert.Contains(t, w.Body.String(), tc.detail)
		})
	}
}

func TestPutAutorunTask_TypeOutOfRange(t *testing.T) {
	ensureTestDB()
	router := taskRouter(t)

	w := doRequest(t, router, "PUT", "/web/autorun/task", map[string]interface{}{
		"type": dbTable.AutorunTypeMax + 1, "scope": []string{"ALL"}, "priority": 1,
		"entries": []map[string]interface{}{
			{"action": map[string]interface{}{"useDate": "2026-09-14"}},
		},
	})

	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "type 必须为 0-5")
}
