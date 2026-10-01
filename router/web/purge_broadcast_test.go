package web

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"AstraScheduleServerGo/db"
	"AstraScheduleServerGo/model/dbTable"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedScheduleClasses 准备一个干净的 schedules 表并写入给定班级行（写入 default 命名空间）。
// 业务代码读写的是包级 db.GetDB() 实例，必须复用包内初始化在同一个实例上建表，
// 否则单独跑本文件时 client_configs 等表不存在（sqlite :memory: 每连接一个库）。
func seedScheduleClasses(t *testing.T, classes ...[3]string) {
	t.Helper()
	ensureTestDB()
	conn := db.GetDB()
	require.NoError(t, conn.Where("1 = 1").Delete(&dbTable.Schedule{}).Error)
	for _, class := range classes {
		require.NoError(t, conn.Create(&dbTable.Schedule{
			Namespace: "default", School: class[0], Grade: class[1], Class: class[2],
		}).Error)
	}
}

// 粗作用域必须落到班级粒度：边缘的键是班级粒度，不能按前缀删。
func TestPurgeScopesOfScopeExpandsAllGranularities(t *testing.T) {
	seedScheduleClasses(t,
		[3]string{"s1", "g1", "1"},
		[3]string{"s1", "g1", "2"},
		[3]string{"s1", "g2", "1"},
		[3]string{"s2", "g1", "1"},
	)
	// 其它命名空间的行不能出现在本租户的失效声明里
	require.NoError(t, db.GetDB().Create(&dbTable.Schedule{
		Namespace: "other", School: "s1", Grade: "g1", Class: "9",
	}).Error)

	all := []string{"s1/g1/1", "s1/g1/2", "s1/g2/1", "s2/g1/1"}
	cases := []struct {
		scope string
		want  []string
	}{
		{"ALL", all},
		{"", all},
		{"s1", []string{"s1/g1/1", "s1/g1/2", "s1/g2/1"}},
		{"s1/g1", []string{"s1/g1/1", "s1/g1/2"}},
		{"s1/g1/2", []string{"s1/g1/2"}},
		{"s9", nil},
	}
	for _, tc := range cases {
		assert.ElementsMatch(t, tc.want, purgeScopesOfScope("default", tc.scope), "scope=%q", tc.scope)
	}
}

// 同一班级被多个作用域覆盖时只声明一次。
func TestPurgeScopesOfScopesDeduplicates(t *testing.T) {
	seedScheduleClasses(t,
		[3]string{"s1", "g1", "1"},
		[3]string{"s1", "g1", "2"},
	)

	got := purgeScopesOfScopes("default", []string{"s1/g1", "s1/g1/1", "ALL"})

	assert.ElementsMatch(t, []string{"s1/g1/1", "s1/g1/2"}, got)
}

// broadcastScopes 是所有写路径的统一出口：除了广播，还必须带上失效声明。
func TestBroadcastScopesDeclaresPurgeHeader(t *testing.T) {
	seedScheduleClasses(t,
		[3]string{"s1", "g1", "1"},
		[3]string{"s1", "g1", "2"},
		[3]string{"s1", "g2", "1"},
	)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	broadcastScopes(c, "default", []string{"s1/g1"})

	assert.ElementsMatch(t, []string{"s1/g1/1", "s1/g1/2"},
		strings.Split(w.Header().Get(purgeScopesHeader), ","))
}

// 全量导入这类粗作用域不能把响应头撑爆：超过上限时截断。
// 删除学校/年级会直接调 setPurgeScopes，所以上限必须兜在写头的那一个出口上。
func TestPurgeScopesOfScopesCapsHeaderSize(t *testing.T) {
	classes := make([][3]string, 0, maxPurgeScopes+5)
	for i := 0; i < maxPurgeScopes+5; i++ {
		classes = append(classes, [3]string{"s1", "g1", strconv.Itoa(i + 1)})
	}
	seedScheduleClasses(t, classes...)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	broadcastScopes(c, "default", []string{"ALL"})

	header := strings.Split(w.Header().Get(purgeScopesHeader), ",")
	require.Len(t, header, maxPurgeScopes)
	assert.Contains(t, header, "s1/g1/1")
}

// 删除年级会把班级行一起带走：失效范围必须在删除前取好，否则缓存永远停在旧课表。
func TestDeleteGradeDeclaresPurgeScopeBeforeRemoval(t *testing.T) {
	seedScheduleClasses(t,
		[3]string{"s1", "g1", "1"},
		[3]string{"s1", "g1", "2"},
		[3]string{"s1", "g2", "1"},
	)
	router := gin.New()
	router.DELETE("/web/schools/:school/grades/:grade", adminOnly(t), DeleteGrade)

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodDelete, "/web/schools/s1/grades/g1", nil)
	require.NoError(t, err)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.ElementsMatch(t, []string{"s1/g1/1", "s1/g1/2"},
		strings.Split(w.Header().Get(purgeScopesHeader), ","))

	var remaining []string
	require.NoError(t, db.GetDB().Model(&dbTable.Schedule{}).Pluck("class", &remaining).Error)
	assert.Equal(t, []string{"1"}, remaining)
}

// 删除学校同理：全校班级的缓存都要失效。
func TestDeleteSchoolDeclaresPurgeScopeBeforeRemoval(t *testing.T) {
	seedScheduleClasses(t,
		[3]string{"s1", "g1", "1"},
		[3]string{"s1", "g2", "3"},
		[3]string{"s2", "g1", "1"},
	)
	router := gin.New()
	router.DELETE("/web/schools/:school", adminOnly(t), DeleteSchool)

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodDelete, "/web/schools/s1", nil)
	require.NoError(t, err)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.ElementsMatch(t, []string{"s1/g1/1", "s1/g2/3"},
		strings.Split(w.Header().Get(purgeScopesHeader), ","))
}
