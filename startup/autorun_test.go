package startup

import (
	"AstraScheduleServerGo/db"
	"AstraScheduleServerGo/model"
	"AstraScheduleServerGo/model/dbTable"
	"AstraScheduleServerGo/testutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	// InitTestDB 只负责设置 model.Configs，它建的表在它自己的连接上；
	// 业务代码走 db.GetDB() 的单例连接，两者是不同的 :memory: 库，因此这里用业务单例建表。
	testutil.InitTestDB()
	if err := db.GetDB().AutoMigrate(&dbTable.AutorunRecord{}); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// seedExpiredRecord 造一条已过期的任务；CreatedAt 显式设为过去，绕过 GORM 的自动时间戳，
// 以便同时覆盖「保留期」与「已过期」两个条件
func seedExpiredRecord(t *testing.T, hashID string, createdAt time.Time) {
	t.Helper()
	record := dbTable.AutorunRecord{
		HashID:    hashID,
		Name:      hashID,
		EType:     dbTable.AutorunTypeSchedule,
		Scope:     []string{"school/grade/class"},
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
		Entries: []dbTable.AutorunEntry{{
			ID:   "entry-1",
			When: &dbTable.AutorunCondition{Kind: dbTable.AutorunWhenDate, Date: "2026-01-01"},
			Action: map[string]interface{}{
				"schedule": map[string]interface{}{"periods": []interface{}{}},
			},
		}},
	}
	require.NoError(t, db.GetDB().Create(&record).Error)
}

func resetCleanerState(t *testing.T) {
	t.Helper()
	autorunCleanMu.Lock()
	autorunLastCleaned = time.Time{}
	autorunCleanMu.Unlock()
	model.Configs.Autorun.AutoClean = false
	require.NoError(t, db.GetDB().Where("1 = 1").Delete(&dbTable.AutorunRecord{}).Error)
}

// waitForInlineCleanup 轮询等待后台清理落地
func waitForInlineCleanup(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		rows, err := db.FetchAutorunRecords("")
		require.NoError(t, err)
		if len(rows) == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestMaybeCleanExpiredAutorun_SkipsWhenDisabled(t *testing.T) {
	resetCleanerState(t)
	seedExpiredRecord(t, "keep-me", time.Now().AddDate(0, 0, -60))

	MaybeCleanExpiredAutorun()
	// 关闭时不应产生任何后台清理：留出窗口后数据仍在
	time.Sleep(150 * time.Millisecond)

	rows, err := db.FetchAutorunRecords("")
	require.NoError(t, err)
	assert.Len(t, rows, 1, "auto_clean 关闭时不应清理")
}

func TestMaybeCleanExpiredAutorun_CleansWhenEnabled(t *testing.T) {
	resetCleanerState(t)
	seedExpiredRecord(t, "clean-me", time.Now().AddDate(0, 0, -60))

	model.Configs.Autorun.AutoClean = true
	MaybeCleanExpiredAutorun()
	waitForInlineCleanup(t)

	rows, err := db.FetchAutorunRecords("")
	require.NoError(t, err)
	assert.Empty(t, rows, "auto_clean 开启时后台应清理已过期任务")
}

func TestMaybeCleanExpiredAutorun_KeepsFreshRecords(t *testing.T) {
	resetCleanerState(t)
	// 创建时间在保留期内：即便已过期，自动清理也不应动它（避免刚补录就被删）
	seedExpiredRecord(t, "fresh", time.Now())

	model.Configs.Autorun.AutoClean = true
	MaybeCleanExpiredAutorun()
	time.Sleep(300 * time.Millisecond)

	rows, err := db.FetchAutorunRecords("")
	require.NoError(t, err)
	assert.Len(t, rows, 1, "保留期内的记录不应被自动清理")
}

func TestMaybeCleanExpiredAutorun_RespectsInterval(t *testing.T) {
	resetCleanerState(t)
	model.Configs.Autorun.AutoClean = true

	MaybeCleanExpiredAutorun()
	autorunCleanMu.Lock()
	first := autorunLastCleaned
	autorunCleanMu.Unlock()
	require.False(t, first.IsZero(), "首次调用应触发清理")

	MaybeCleanExpiredAutorun()
	autorunCleanMu.Lock()
	second := autorunLastCleaned
	autorunCleanMu.Unlock()
	assert.Equal(t, first, second, "间隔内的重复调用不应再次触发清理")
}
