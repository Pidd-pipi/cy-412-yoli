package repository

import (
	"errors"
	"time"

	"github.com/smartestate/smartestate/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type VisitorPassRepository struct{ DB *gorm.DB }

func NewVisitorPassRepository(db *gorm.DB) *VisitorPassRepository { return &VisitorPassRepository{db} }

func (r *VisitorPassRepository) Create(v *model.VisitorPass) error { return r.DB.Create(v).Error }

func (r *VisitorPassRepository) List(userID uint, status string) (out []model.VisitorPass, e error) {
	q := r.DB.Preload("User").Preload("Reviewer").Order("created_at desc")
	if userID > 0 {
		q = q.Where("user_id = ?", userID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	e = q.Find(&out).Error
	return
}

func (r *VisitorPassRepository) ByID(id uint) (v model.VisitorPass, e error) {
	e = r.DB.Preload("User").Preload("Reviewer").First(&v, id).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		e = ErrNotFound
	}
	return
}

func (r *VisitorPassRepository) Update(v *model.VisitorPass) error { return r.DB.Save(v).Error }

// FindEffectiveOverlap 返回同一标准化车牌在 [start,end) 上重叠、且当前仍占有时段的有效通行证。
// 已撤销/已拒绝/已过期（end_at <= now）的通行证不占用时段，因此不会冲突。
// 使用 FOR UPDATE 锁定命中行：在 MySQL/InnoDB 下，并发审批事务会阻塞到先到者提交，
// 随后读到最新的 approved 行并判冲突；SQLite 下该子句被忽略（写入天然串行）。
func (r *VisitorPassRepository) FindEffectiveOverlap(tx *gorm.DB, plate string, start, end, now time.Time, excludeID uint) (v model.VisitorPass, found bool, e error) {
	q := tx.Preload("User").
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("normalized_plate = ?", plate).
		Where("status = ?", "approved").
		Where("end_at > ?", now).
		Where("start_at < ? AND end_at > ?", end, start)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	err := q.Order("start_at asc").First(&v).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return v, false, nil
	}
	if err != nil {
		return v, false, err
	}
	return v, true, nil
}

// ApproveInTx 在事务内把指定申请从 pending 条件更新为 approved；仅当 RowsAffected=1 时成功，
// 从而保证重复审批 / 并发审批最多成功一次（由数据库行级更新兜底，不依赖应用层时序）。
func (r *VisitorPassRepository) ApproveInTx(tx *gorm.DB, id, reviewerID uint, reviewedAt time.Time) (int64, error) {
	res := tx.Model(&model.VisitorPass{}).
		Where("id = ? AND status = ?", id, "pending").
		Updates(map[string]any{
			"status":         "approved",
			"reviewer_id":    reviewerID,
			"reviewed_at":    reviewedAt,
			"failure_reason": "",
			"updated_at":     reviewedAt,
		})
	return res.RowsAffected, res.Error
}

// MarkConflictInTx 冲突时整次拒绝但保持待审：记录失败原因与最近一次审批尝试，状态不变。
func (r *VisitorPassRepository) MarkConflictInTx(tx *gorm.DB, id, reviewerID uint, reason string, at time.Time) error {
	return tx.Model(&model.VisitorPass{}).Where("id = ?", id).
		Updates(map[string]any{"failure_reason": reason, "reviewer_id": reviewerID, "reviewed_at": at, "updated_at": at}).Error
}

// RejectInTx 人工拒绝：仅 pending 申请可被拒绝，返回是否实际更新。
func (r *VisitorPassRepository) RejectInTx(tx *gorm.DB, id, reviewerID uint, reason string, at time.Time) (int64, error) {
	res := tx.Model(&model.VisitorPass{}).
		Where("id = ? AND status = ?", id, "pending").
		Updates(map[string]any{
			"status":         "rejected",
			"reviewer_id":    reviewerID,
			"reviewed_at":    at,
			"failure_reason": reason,
			"updated_at":     at,
		})
	return res.RowsAffected, res.Error
}

// RevokeInTx 撤销：仅 approved 且到访开始时间晚于 now 的通行证可撤销，返回是否实际更新。
func (r *VisitorPassRepository) RevokeInTx(tx *gorm.DB, id, operatorID uint, now time.Time) (int64, error) {
	res := tx.Model(&model.VisitorPass{}).
		Where("id = ? AND status = ? AND start_at > ?", id, "approved", now).
		Updates(map[string]any{
			"status":        "revoked",
			"revoked_by_id": operatorID,
			"revoked_at":    now,
			"updated_at":    now,
		})
	return res.RowsAffected, res.Error
}

func (r *VisitorPassRepository) CountByStatus(status string) (int64, error) {
	var n int64
	e := r.DB.Model(&model.VisitorPass{}).Where("status = ?", status).Count(&n).Error
	return n, e
}
