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

// FetchAutorunRecords 获取自动任务记录（无命名空间，向后兼容）
func FetchAutorunRecords(hashid string) ([]dbTable.AutorunRecord, error) {
	return FetchAutorunRecordsNs("", hashid)
}

// FetchAutorunRecordsNs 获取自动任务记录（带命名空间）
// 安全修复：namespace 为空时不查询，避免跨租户返回数据
func FetchAutorunRecordsNs(namespace, hashid string) ([]dbTable.AutorunRecord, error) {
	records := make([]dbTable.AutorunRecord, 0)
	if namespace == "" {
		return records, nil
	}
	q := GetDB().Model(&dbTable.AutorunRecord{}).Where("namespace = ?", namespace)
	if hashid != "" {
		q = q.Where(hashIDWhere, hashid)
	}
	err := q.Find(&records).Error
	return records, err
}

// FetchAutorunRecordNamespacesByHash 按 hash_id 跨 namespace 查询记录所属的命名空间。
// 仅用于写入前的归属校验：主键是全局唯一的 hash_id，客户端若自带一个已属于其它租户的 ID，
// Upsert(UpdateAll) 会连 Namespace 一起改写，必须在校验后拒绝。
func FetchAutorunRecordNamespacesByHash(hashid string) ([]string, error) {
	out := make([]string, 0)
	if hashid == "" {
		return out, nil
	}
	err := GetDB().Model(&dbTable.AutorunRecord{}).
		Where(hashIDWhere, hashid).
		Pluck("namespace", &out).Error
	return out, err
}

// DeleteAutorunRecord 删除自动任务记录（无命名空间，向后兼容）
func DeleteAutorunRecord(hashid string) (int64, error) {
	return DeleteAutorunRecordNs("", hashid)
}

// DeleteAutorunRecordNs 删除自动任务记录（带命名空间）
// 安全修复：namespace 为空时不删除，避免跨租户删除
func DeleteAutorunRecordNs(namespace, hashid string) (int64, error) {
	if namespace == "" {
		return 0, nil
	}
	resp := GetDB().Where(hashIDWhere, hashid).Where("namespace = ?", namespace).Delete(&dbTable.AutorunRecord{})
	return resp.RowsAffected, resp.Error
}

// DeleteAutorunRecordNsTx 在给定连接上按命名空间删除任务：与版本推进同事务时传入 tx
func DeleteAutorunRecordNsTx(tx *gorm.DB, namespace, hashid string) (int64, error) {
	if namespace == "" {
		return 0, nil
	}
	resp := tx.Where(hashIDWhere, hashid).Where("namespace = ?", namespace).Delete(&dbTable.AutorunRecord{})
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

// expiredCleanable 判定一条任务是否属于「已过期且可清理」。
// 任务级停用、含停用条目（TaskStatus 会跳过停用条目，整条删除会丢掉用户保存的配置）、
// 以及创建时间仍在保留期内的任务一律保留。
func expiredCleanable(record dbTable.AutorunRecord, today time.Time, cutoff time.Time) bool {
	if record.Disabled || hasDisabledEntry(record) {
		return false
	}
	if service.TaskStatus(record, today) != autorunStatusExpired {
		return false
	}
	return cutoff.IsZero() || !record.CreatedAt.After(cutoff)
}

// DeleteExpiredAutorunRecordsNs 删除某个命名空间内「已过期且未停用」的自动任务，
// 返回删除数量与被删作用域。minAge > 0 时只清创建时间早于 now-minAge 的记录：
// 自动清理用这个门槛，避免刚补录就过期的历史任务立刻被清掉。
// 判定不依赖 status 列（那只是列表展示的派生缓存），而是逐条按当前时间求值；
// 读与删在同一事务里且条件一致，并发下被停用或改期的记录不会被误删。
func DeleteExpiredAutorunRecordsNs(namespace string, today time.Time, minAge time.Duration) (int64, []string, error) {
	if namespace == "" {
		return 0, nil, nil
	}
	cutoff := time.Time{}
	if minAge > 0 {
		cutoff = today.Add(-minAge)
	}
	scopes := make([]string, 0)
	deleted := int64(0)
	err := GetDB().Transaction(func(tx *gorm.DB) error {
		records := make([]dbTable.AutorunRecord, 0)
		if err := tx.Where("namespace = ?", namespace).Find(&records).Error; err != nil {
			return err
		}
		ids := make([]string, 0, len(records))
		for _, record := range records {
			if !expiredCleanable(record, today, cutoff) {
				continue
			}
			ids = append(ids, record.HashID)
			scopes = append(scopes, record.Scope...)
		}
		if len(ids) == 0 {
			return nil
		}
		resp := tx.Where("namespace = ?", namespace).Where("disabled = ?", false).
			Where("hash_id IN ?", ids).Delete(&dbTable.AutorunRecord{})
		deleted = resp.RowsAffected
		if resp.Error != nil {
			return resp.Error
		}
		if deleted > 0 {
			// 与删除同事务推进版本：清理的是「难以精确到班」的作用域，用全局版本兜底
			if err := BumpDataVersion(tx, namespace, "", "", "", today); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, nil, err
	}
	return deleted, scopes, nil
}

// ExpiredCleanupResult 一次跨命名空间清理中，某个命名空间被删记录的作用域
// （调用方据此按租户广播刷新）
type ExpiredCleanupResult struct {
	Namespace string
	Scopes    []string
	Deleted   int64
}

// CleanExpiredAutorunRecords 遍历所有命名空间清理已过期任务，返回每个命名空间的删除结果
func CleanExpiredAutorunRecords(today time.Time, minAge time.Duration) ([]ExpiredCleanupResult, error) {
	namespaces := make([]string, 0)
	if err := GetDB().Model(&dbTable.AutorunRecord{}).Distinct().Pluck("namespace", &namespaces).Error; err != nil {
		return nil, err
	}
	results := make([]ExpiredCleanupResult, 0, len(namespaces))
	for _, namespace := range namespaces {
		deleted, scopes, err := DeleteExpiredAutorunRecordsNs(namespace, today, minAge)
		if err != nil {
			return results, err
		}
		if deleted == 0 {
			continue
		}
		results = append(results, ExpiredCleanupResult{Namespace: namespace, Scopes: scopes, Deleted: deleted})
	}
	return results, nil
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

// RefreshAutorunStatuses 刷新自动任务状态（无命名空间，向后兼容）
func RefreshAutorunStatuses(today time.Time) (int64, error) {
	return RefreshAutorunStatusesNs("", today)
}

// RefreshAutorunStatusesNs 刷新自动任务状态（带命名空间）
func RefreshAutorunStatusesNs(namespace string, today time.Time) (int64, error) {
	records, err := FetchAutorunRecordsNs(namespace, "")
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
