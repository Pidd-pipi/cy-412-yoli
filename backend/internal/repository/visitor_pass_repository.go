package repository

import (
	"errors"
	"github.com/smartestate/smartestate/internal/constants"
	"github.com/smartestate/smartestate/internal/model"
	"gorm.io/gorm"
	"time"
)

type VisitorPassRepository struct{ DB *gorm.DB }

func NewVisitorPassRepository(db *gorm.DB) *VisitorPassRepository  { return &VisitorPassRepository{db} }
func (r *VisitorPassRepository) Create(v *model.VisitorPass) error { return r.DB.Create(v).Error }
func (r *VisitorPassRepository) List(userID uint, status string) (out []model.VisitorPass, e error) {
	q := r.DB.Preload("User").Order("created_at desc")
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
	e = r.DB.Preload("User").First(&v, id).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		e = ErrNotFound
	}
	return
}
func (r *VisitorPassRepository) ActiveOverlapping(plate string, start, end, now time.Time, excludeID uint) (out []model.VisitorPass, e error) {
	e = r.DB.Where("plate_no = ? AND status = ? AND id <> ? AND visit_end > ? AND visit_start < ? AND visit_end > ?",
		plate, constants.VisitorPassStatusApproved, excludeID, now, end, start).Find(&out).Error
	return
}
func (r *VisitorPassRepository) SaveFailReason(id uint, reason string, now time.Time) error {
	return r.DB.Model(&model.VisitorPass{}).Where("id = ?", id).Updates(map[string]any{"fail_reason": reason, "updated_at": now}).Error
}
func (r *VisitorPassRepository) ClaimApproved(id, approverID uint, now time.Time) (int64, error) {
	tx := r.DB.Model(&model.VisitorPass{}).Where("id = ? AND status = ?", id, constants.VisitorPassStatusPending).
		Updates(map[string]any{"status": constants.VisitorPassStatusApproved, "approved_by": approverID, "approved_at": now, "fail_reason": "", "updated_at": now})
	return tx.RowsAffected, tx.Error
}
func (r *VisitorPassRepository) MarkRevoked(id uint, now time.Time) (int64, error) {
	tx := r.DB.Model(&model.VisitorPass{}).Where("id = ? AND status IN ?", id, []string{constants.VisitorPassStatusPending, constants.VisitorPassStatusApproved}).
		Updates(map[string]any{"status": constants.VisitorPassStatusRevoked, "revoked_at": now, "updated_at": now})
	return tx.RowsAffected, tx.Error
}
func (r *VisitorPassRepository) ExpireStale(now time.Time) error {
	return r.DB.Model(&model.VisitorPass{}).Where("status = ? AND visit_end <= ?", constants.VisitorPassStatusApproved, now).
		Updates(map[string]any{"status": constants.VisitorPassStatusExpired, "updated_at": now}).Error
}
