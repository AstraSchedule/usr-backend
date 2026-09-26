package web

import (
	"AstraScheduleServerGo/db"
	"AstraScheduleServerGo/model/dbTable"
	"AstraScheduleServerGo/testutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 删除操作不会留下任何时间戳（行连同 UpdatedAt 一起消失），因此必须显式推进数据版本；
// 否则客户端会一直命中 304，继续展示已删除的配置。
// 注意：testutil.InitTestDB 只负责设置 model.Configs，它建的表在自己的连接上，
// 业务代码走 db.GetDB() 的单例连接，所以这里用业务单例建表。
func TestBumpDataVersionForDeletedScopes(t *testing.T) {
	testutil.InitTestDB()
	require.NoError(t, db.GetDB().AutoMigrate(&dbTable.DataVersion{}))
	require.NoError(t, db.GetDB().Where("1 = 1").Delete(&dbTable.DataVersion{}).Error)

	// 班级级作用域：只推进该班，不动全局
	bumpDataVersionForDeletedScopes([]string{"s1/g1/c1"})
	assert.False(t, db.GetDataVersion("s1", "g1", "c1").Version.IsZero(), "班级级删除应推进该班版本")
	assert.True(t, db.GetDataVersion("", "", "").Version.IsZero(), "班级级删除不应动全局版本")

	// 年级级作用域：无法枚举到具体班级，退化为全局版本
	bumpDataVersionForDeletedScopes([]string{"s1/g1"})
	assert.False(t, db.GetDataVersion("", "", "").Version.IsZero(), "粗作用域删除应推进全局版本")

	// 重复项与空输入不应出错（幂等）
	bumpDataVersionForDeletedScopes([]string{"s1/g1/c1", "s1/g1/c1", ""})
	bumpDataVersionForDeletedScopes(nil)
}
