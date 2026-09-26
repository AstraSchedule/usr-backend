package startup

import (
	"AstraScheduleServerGo/db"
	"AstraScheduleServerGo/model"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// autorunCleanInterval 自动清理已过期自动任务的最小间隔
const autorunCleanInterval = 24 * time.Hour

var (
	autorunCleanMu     sync.Mutex
	autorunLastCleaned time.Time
)

// StartAutorunCleaner 启动时预约一次清理（实际清理在后台执行，不阻塞启动）
func StartAutorunCleaner() {
	MaybeCleanExpiredAutorun()
}

// MaybeCleanExpiredAutorun 请求驱动的惰性清理：距上次清理超过间隔时，
// 在后台清理一次并立即返回，不阻塞当前请求。
// serverless 没有常驻进程，ticker 不可靠；请求触发在常驻与 serverless 下都成立。
func MaybeCleanExpiredAutorun() {
	if !model.Configs.Autorun.AutoClean {
		return
	}
	autorunCleanMu.Lock()
	// 零值表示本次进程内还没清理过：启动后的第一个请求即触发，不必等满一个间隔
	if !autorunLastCleaned.IsZero() && time.Since(autorunLastCleaned) < autorunCleanInterval {
		autorunCleanMu.Unlock()
		return
	}
	autorunLastCleaned = time.Now()
	autorunCleanMu.Unlock()
	go cleanExpiredAutorun()
}

func cleanExpiredAutorun() {
	deleted, _, err := db.DeleteExpiredAutorunRecords(time.Now())
	if err != nil {
		logrus.Warnf("自动清理已过期自动任务失败: %v", err)
		return
	}
	if deleted > 0 {
		logrus.Infof("自动清理已过期自动任务：删除 %d 条", deleted)
	}
}
