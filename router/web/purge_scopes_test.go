package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"AstraScheduleServerGo/db"
	"AstraScheduleServerGo/model/dbTable"
	"AstraScheduleServerGo/testutil"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 边缘按这个头失效课表版本缓存；头的格式与键的粒度必须稳定。
func TestSetPurgeScopesHeaderFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	setPurgeScopes(c, []string{"s1/g1/c1", " s1/g1/c2 ", ""})

	assert.Equal(t, "s1/g1/c1,s1/g1/c2", w.Header().Get(purgeScopesHeader))
}

// 空列表不设置该头：没有失效声明等价于「这次写入与缓存无关」。
func TestSetPurgeScopesEmptyLeavesHeaderAbsent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	setPurgeScopes(c, nil)
	setPurgeScopes(c, []string{"", "   "})

	assert.Empty(t, w.Header().Get(purgeScopesHeader))
}

// 年级级改动必须展开成该年级下所有班级：边缘的键是班级粒度。
func TestPurgeScopesOfGradeExpandsClasses(t *testing.T) {
	testutil.InitTestDB()
	conn := db.GetDB()
	require.NoError(t, conn.AutoMigrate(&dbTable.Schedule{}))
	require.NoError(t, conn.Where("1 = 1").Delete(&dbTable.Schedule{}).Error)

	for _, class := range []string{"1", "2", ""} {
		require.NoError(t, conn.Create(&dbTable.Schedule{
			School: "s1", Grade: "g1", Class: class,
		}).Error)
	}
	require.NoError(t, conn.Create(&dbTable.Schedule{
		School: "s1", Grade: "g2", Class: "1",
	}).Error)

	got := purgeScopesOfGrade("s1", "g1")

	assert.ElementsMatch(t, []string{"s1/g1/1", "s1/g1/2"}, got)
}

// 端到端：保存课表成功后，响应里必须带上该班的失效声明。
func TestPutScheduleConfigDeclaresPurgeScope(t *testing.T) {
	testutil.InitTestDB()
	// adminOnly 需要 users 表（它会在库内建一个测试管理员）
	require.NoError(t, db.GetDB().AutoMigrate(&dbTable.User{}, &dbTable.Schedule{}, &dbTable.Timetable{}))

	daily := make([]map[string]interface{}, 7)
	for i := range daily {
		daily[i] = map[string]interface{}{"Chinese": "一", "English": "Mon", "timetable": "", "classList": []interface{}{}}
	}
	payload, err := json.Marshal(map[string]interface{}{"daily_class": daily})
	require.NoError(t, err)

	router := gin.New()
	router.PUT("/web/config/:school/:grade/:class_number/schedule", adminOnly(t), PutScheduleConfig)

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPut, "/web/config/s1/g1/1/schedule", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "s1/g1/1", w.Header().Get(purgeScopesHeader))
}
