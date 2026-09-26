package db

import (
	"AstraScheduleServerGo/model/dbTable"
	"AstraScheduleServerGo/service"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const hashIDWhere = "hash_id = ?"

// autorunStatusExpired 任务已过期（与 service.TaskStatus 的返回值 2 一致）
const autorunStatusExpired = 2

func FetchAutorunRecords(hashid string) ([]dbTable.AutorunRecord, error) {
	records := make([]dbTable.AutorunRecord, 0)
	q := GetDB().Model(&dbTable.AutorunRecord{})
	if hashid != "" {
		q = q.Where(hashIDWhere, hashid)
	}
	err := q.Find(&records).Error
	return records, err
}

func DeleteAutorunRecord(hashid string) (int64, error) {
	resp := GetDB().Where(hashIDWhere, hashid).Delete(&dbTable.AutorunRecord{})
	return resp.RowsAffected, resp.Error
}

// hasDisabledEntry 记录里是否含被停用的条目。TaskStatus 会跳过停用条目，
// 所以「启用条目已结束 + 还有停用条目」的记录同样会被判为已过期；
// 这类记录不能删，否则丢的是用户停用保留的配置。
func hasDisabledEntry(record dbTable.AutorunRecord) bool {
	for _, entry := range service.EntriesOf(record) {
		if entry.Disabled {
			return true
		}
	}
	return false
}

// DeleteExpiredAutorunRecords 删除「已过期且未停用」的自动任务，
// 返回删除数量与被删记录涉及的作用域（供调用方广播刷新）。
// status 是派生缓存，删除前先按当前时间刷新一次，避免拿陈旧状态做判断。
// 查询与删除放在同一事务里并保持条件一致：并发下被停用或改期的记录不会被误删。
func DeleteExpiredAutorunRecords(today time.Time) (int64, []string, error) {
	if _, err := RefreshAutorunStatuses(today); err != nil {
		return 0, nil, err
	}
	scopes := make([]string, 0)
	deleted := int64(0)
	err := GetDB().Transaction(func(tx *gorm.DB) error {
		expired := make([]dbTable.AutorunRecord, 0)
		if err := tx.Where("disabled = ? AND status = ?", false, autorunStatusExpired).
			Find(&expired).Error; err != nil {
			return err
		}
		ids := make([]string, 0, len(expired))
		for _, record := range expired {
			if hasDisabledEntry(record) {
				continue
			}
			ids = append(ids, record.HashID)
			scopes = append(scopes, record.Scope...)
		}
		if len(ids) == 0 {
			return nil
		}
		resp := tx.Where("disabled = ? AND status = ?", false, autorunStatusExpired).
			Where("hash_id IN ?", ids).Delete(&dbTable.AutorunRecord{})
		deleted = resp.RowsAffected
		return resp.Error
	})
	if err != nil {
		return 0, nil, err
	}
	return deleted, scopes, nil
}

func UpsertAutorunRecord(record *dbTable.AutorunRecord) error {
	return GetDB().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "hash_id"}},
		UpdateAll: true,
	}).Create(record).Error
}

// deriveStatusForRecord 推导任务状态：0 待生效 / 1 生效中 / 2 已过期。
// v2 起按任务内所有条目的生效区间（并集）判定，v1 的单日规则退化为同一天内生效，结果不变。
func deriveStatusForRecord(record dbTable.AutorunRecord, today time.Time) int {
	if record.EType < dbTable.AutorunTypeCompensation || record.EType > dbTable.AutorunTypeMax {
		return 0
	}
	return service.TaskStatus(record, today)
}

func RefreshAutorunStatuses(today time.Time) (int64, error) {
	records, err := FetchAutorunRecords("")
	if err != nil {
		return 0, err
	}
	updated := int64(0)
	for i := range records {
		newStatus := deriveStatusForRecord(records[i], today)
		if newStatus == records[i].Status {
			continue
		}
		// status 只是给管理端列表展示的派生缓存，客户端响应由读取时按时间重新求值，
		// 不依赖该列；因此这里用 UpdateColumn 跳过时间戳维护——否则每次状态翻转都会推进
		// 自动任务记录的 UpdatedAt，进而推进课表版本，造成一次无意义的重新拉取
		if err := GetDB().Model(&dbTable.AutorunRecord{}).
			Where(hashIDWhere, records[i].HashID).
			UpdateColumn("status", newStatus).Error; err != nil {
			return updated, err
		}
		updated++
	}
	return updated, nil
}
