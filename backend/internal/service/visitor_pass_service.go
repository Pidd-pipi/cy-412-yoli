package service

import (
	"errors"
	"fmt"
	"github.com/smartestate/smartestate/internal/constants"
	"github.com/smartestate/smartestate/internal/model"
	"github.com/smartestate/smartestate/internal/repository"
	"github.com/smartestate/smartestate/internal/util"
	"log/slog"
	"strings"
	"sync"
	"time"
)

var (
	ErrVisitorPassConflict   = errors.New("visitor pass time conflict")
	ErrVisitorPassNotPending = errors.New("visitor pass not pending")
)

type VisitorPassService struct {
	repo      *repository.VisitorPassRepository
	logger    *slog.Logger
	approveMu sync.Mutex
}

func NewVisitorPassService(r *repository.VisitorPassRepository, l *slog.Logger) *VisitorPassService {
	return &VisitorPassService{repo: r, logger: l}
}
func (s *VisitorPassService) Create(uid uint, plate, building, unit, room string, start, end time.Time, role string) (model.VisitorPass, error) {
	plate = strings.ToUpper(strings.TrimSpace(plate))
	if !end.After(start) {
		return model.VisitorPass{}, fmt.Errorf("VisitorPass[plate=%s] create failed: visit_end must be after visit_start, current role=%s", plate, role)
	}
	if !end.After(time.Now()) {
		return model.VisitorPass{}, fmt.Errorf("VisitorPass[plate=%s] create failed: visit window already expired, current role=%s", plate, role)
	}
	v := model.VisitorPass{UserID: uid, PlateNo: plate, Building: building, Unit: unit, Room: room, VisitStart: start, VisitEnd: end, Status: constants.VisitorPassStatusPending}
	if e := s.repo.Create(&v); e != nil {
		return v, fmt.Errorf("VisitorPass[user_id=%d] create failed: %w", uid, e)
	}
	return s.repo.ByID(v.ID)
}
func (s *VisitorPassService) List(uid uint, role, status string) ([]model.VisitorPass, error) {
	if status != "" && !constants.ValidVisitorPassStatuses[status] {
		return nil, fmt.Errorf("VisitorPass list failed: invalid status=%s, current role=%s", status, role)
	}
	if e := s.repo.ExpireStale(time.Now()); e != nil {
		s.logger.Error("expire stale visitor passes", "error", e)
	}
	if role == constants.UserRoleResident {
		return s.repo.List(uid, status)
	}
	return s.repo.List(0, status)
}
func (s *VisitorPassService) Approve(id, approverID uint, role string) (model.VisitorPass, error) {
	s.approveMu.Lock()
	defer s.approveMu.Unlock()
	now := time.Now()
	if e := s.repo.ExpireStale(now); e != nil {
		s.logger.Error("expire stale visitor passes", "error", e)
	}
	v, e := s.repo.ByID(id)
	if e != nil {
		return v, fmt.Errorf("VisitorPass[id=%d] approve failed: application not found, current role=%s: %w", id, role, e)
	}
	if v.Status != constants.VisitorPassStatusPending {
		return v, fmt.Errorf("VisitorPass[id=%d] approve failed: already %s, repeated approval rejected, current role=%s: %w", id, v.Status, role, ErrVisitorPassNotPending)
	}
	if !v.VisitEnd.After(now) {
		return v, fmt.Errorf("VisitorPass[id=%d] approve failed: visit window already expired, current role=%s", id, role)
	}
	conflicts, e := s.repo.ActiveOverlapping(v.PlateNo, v.VisitStart, v.VisitEnd, now, v.ID)
	if e != nil {
		return v, fmt.Errorf("VisitorPass[id=%d] approve failed: %w", id, e)
	}
	if len(conflicts) > 0 {
		reason := fmt.Sprintf("车牌 %s 在 %s~%s 与有效通行证#%d 时段冲突", v.PlateNo, util.Date(v.VisitStart), util.Date(v.VisitEnd), conflicts[0].ID)
		if se := s.repo.SaveFailReason(id, reason, now); se != nil {
			s.logger.Error("save visitor pass fail reason", "error", se)
		}
		return v, fmt.Errorf("VisitorPass[id=%d] approve conflict: %s, current role=%s: %w", id, reason, role, ErrVisitorPassConflict)
	}
	n, e := s.repo.ClaimApproved(id, approverID, now)
	if e != nil {
		return v, fmt.Errorf("VisitorPass[id=%d] approve failed: %w", id, e)
	}
	if n == 0 {
		return v, fmt.Errorf("VisitorPass[id=%d] approve failed: concurrent approval already committed, current role=%s: %w", id, role, ErrVisitorPassNotPending)
	}
	return s.repo.ByID(id)
}
func (s *VisitorPassService) Revoke(id, uid uint, role string) (model.VisitorPass, error) {
	v, e := s.repo.ByID(id)
	if e != nil {
		return v, fmt.Errorf("VisitorPass[id=%d] revoke failed: application not found, current role=%s: %w", id, role, e)
	}
	if role == constants.UserRoleResident && v.UserID != uid {
		return v, fmt.Errorf("VisitorPass[id=%d] revoke forbidden: current user=%d owner=%d", id, uid, v.UserID)
	}
	if v.Status != constants.VisitorPassStatusPending && v.Status != constants.VisitorPassStatusApproved {
		return v, fmt.Errorf("VisitorPass[id=%d] revoke failed: status=%s not revocable, current role=%s", id, v.Status, role)
	}
	now := time.Now()
	if !now.Before(v.VisitStart) {
		return v, fmt.Errorf("VisitorPass[id=%d] revoke failed: visit already started at %s, current role=%s", id, util.Date(v.VisitStart), role)
	}
	n, e := s.repo.MarkRevoked(id, now)
	if e != nil {
		return v, fmt.Errorf("VisitorPass[id=%d] revoke failed: %w", id, e)
	}
	if n == 0 {
		return v, fmt.Errorf("VisitorPass[id=%d] revoke failed: concurrent revoke already committed, current role=%s", id, role)
	}
	return s.repo.ByID(id)
}
