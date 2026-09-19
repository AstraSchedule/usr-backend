package client

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"AstraScheduleServerGo/db"
	"AstraScheduleServerGo/model/dbTable"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 调课（type=5）端到端：客户端按「当前日期」取课表时，本端那节课应换成另一端的科目。
// 客户端接口始终用服务端 now，因此这里断言「今天」这一端，另一端取一个固定日期。
func TestGetSchedule_LessonSwap(t *testing.T) {
	ensureTestDB()

	today := time.Now()
	todayIdx := int(today.Weekday())
	// 找一天（1~6 天后）星期与今天不同的日期作为另一端，保证两端落在不同的星期槽位
	offset := 1
	for (todayIdx+offset)%7 == todayIdx {
		offset++
	}
	otherDate := today.AddDate(0, 0, offset)
	otherIdx := int(otherDate.Weekday())

	daily := [7]dbTable.DailyClass{}
	for i := range daily {
		daily[i] = dbTable.DailyClass{Timetable: "常日"}
	}
	daily[todayIdx].ClassList = dbTable.ClassList{{"数"}}
	daily[otherIdx].ClassList = dbTable.ClassList{{"英"}}

	database := db.GetDB()
	// 使用独立 scope，避免与其它测试共享数据
	database.Save(&dbTable.Schedule{School: "swap", Grade: "2024", Class: "1", DailyClasses: daily})
	database.Save(&dbTable.Timetable{
		School: "swap", Grade: "2024",
		TimetableConfig: dbTable.TimetableConfig{
			Timetable: map[string]map[string]interface{}{"常日": {"08:00-08:40": 0}},
			Divider:   map[string][]int{"常日": {}},
			Start:     "2020-09-01",
		},
	})
	// 条件覆盖两端日期（与 dashboard 自动生成的条件一致）
	database.Save(&dbTable.AutorunRecord{
		HashID: "swap-e2e", EType: dbTable.AutorunTypeLessonSwap, Scope: []string{"swap"}, Level: 1,
		Entries: []dbTable.AutorunEntry{{
			ID: "e1",
			When: &dbTable.AutorunCondition{
				Kind: dbTable.AutorunWhenDate, Date: today.Format("2006-01-02"),
			},
			Action: map[string]interface{}{"swap": map[string]interface{}{
				"from": map[string]interface{}{"date": today.Format("2006-01-02"), "period": 1},
				"to":   map[string]interface{}{"date": otherDate.Format("2006-01-02"), "period": 1},
			}},
		}},
	})

	router := setupTestRouter()
	router.GET("/:school/:grade/:class", GetSchedule)

	w := doClientRequest(t, router, "GET", "/swap/2024/1")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	dailyClass, ok := resp["daily_class"].([]interface{})
	require.True(t, ok, "daily_class 应为数组")
	require.Len(t, dailyClass, 7)

	todayCell := classListOf(t, dailyClass, int(today.Weekday()))
	otherCell := classListOf(t, dailyClass, otherIdx)
	assert.Equal(t, []interface{}{"英"}, todayCell, "今天第 1 节应换成另一端的科目")
	assert.Equal(t, []interface{}{"英"}, otherCell, "另一端当天按自己的槽位展示")
}

func classListOf(t *testing.T, dailyClass []interface{}, weekday int) []interface{} {
	t.Helper()
	day, ok := dailyClass[weekday].(map[string]interface{})
	require.True(t, ok, "daily_class 元素应为对象")
	list, ok := day["classList"].([]interface{})
	require.True(t, ok, "classList 应为数组")
	return list
}
