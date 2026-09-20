package service

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/smartestate/smartestate/internal/constants"
	"github.com/smartestate/smartestate/internal/model"
	"github.com/smartestate/smartestate/internal/repository"
	"gorm.io/gorm"
	"log/slog"
)

var (
	ErrVisitorNotFound        = errors.New("visitor pass not found")
	ErrVisitorInvalidTime     = errors.New("visitor pass invalid time window")
	ErrVisitorConflict        = errors.New("visitor pass plate time conflict")
	ErrVisitorAlreadyReviewed = errors.New("visitor pass already reviewed")
	ErrVisitorRevokeWindow    = errors.New("visitor pass cannot revoke after visit start")
	ErrVisitorRevokeState     = errors.New("visitor pass not revocable")
	ErrVisitorForbidden       = errors.New("visitor pass operation forbidden")
)

// plateLocks 按标准化车牌串行化审批，配合数据库条件更新，保证同一车牌重叠时段只可能发出一张通行证。
var (
	plateLocksMu sync.Mutex
	plateLocks   = map[string]*sync.Mutex{}
)

func lockForPlate(plate string) *sync.Mutex {
	plateLocksMu.Lock()
	defer plateLocksMu.Unlock()
	if l, ok := plateLocks[plate]; ok {
		return l
	}
	l := &sync.Mutex{}
	plateLocks[plate] = l
	return l
}

type VisitorPassService struct {
	repo   *repository.VisitorPassRepository
	logger *slog.Logger
}

func NewVisitorPassService(r *repository.VisitorPassRepository, l *slog.Logger) *VisitorPassService {
	return &VisitorPassService{repo: r, logger: l}
}

// NormalizePlate 统一车牌形态：去空白、转大写，使 "粤b12345" 与 "粤B12345" 视为同一车牌。
func NormalizePlate(plate string) string {
	return strings.ToUpper(strings.Join(strings.Fields(plate), ""))
}

func parseVisitWindow(start, end string) (time.Time, time.Time, error) {
	s, e1 := time.Parse(time.RFC3339, start)
	e, e2 := time.Parse(time.RFC3339, end)
	if e1 != nil || e2 != nil {
		return s, e, ErrVisitorInvalidTime
	}
	if !e.After(s) || !s.After(time.Now()) {
		return s, e, ErrVisitorInvalidTime
	}
	return s, e, nil
}

// decorate 计算非持久化的有效态；approved 的通行证按当前时间映射为 upcoming/active/expired。
func decorate(v model.VisitorPass) model.VisitorPass {
	if v.Status == constants.VisitorStatusApproved {
		now := time.Now()
		switch {
		case now.Before(v.StartAt):
			v.EffectiveState = constants.VisitorStateUpcoming
		case now.After(v.EndAt):
			v.EffectiveState = constants.VisitorStateExpired
		default:
			v.EffectiveState = constants.VisitorStateActive
		}
	}
	return v
}

func (s *VisitorPassService) Create(uid uint, plate, visitorName, building, room, startRaw, endRaw string) (model.VisitorPass, error) {
	start, end, e := parseVisitWindow(startRaw, endRaw)
	if e != nil {
		return model.VisitorPass{}, fmt.Errorf("VisitorPass[user_id=%d] create failed: %w, current role=resident", uid, e)
	}
	v := model.VisitorPass{
		UserID:          uid,
		Plate:           strings.TrimSpace(plate),
		NormalizedPlate: NormalizePlate(plate),
		VisitorName:     strings.TrimSpace(visitorName),
		Building:        strings.TrimSpace(building),
		Room:            strings.TrimSpace(room),
		StartAt:         start,
		EndAt:           end,
		Status:          constants.VisitorStatusPending,
	}
	if e := s.repo.Create(&v); e != nil {
		return v, fmt.Errorf("VisitorPass[user_id=%d] create failed: %w", uid, e)
	}
	got, e := s.repo.ByID(v.ID)
	return decorate(got), e
}

func (s *VisitorPassService) List(uid uint, role, status string) ([]model.VisitorPass, error) {
	filterUID := uint(0)
	if role == constants.UserRoleResident {
		filterUID = uid // 业主仅能回读自己的申请
	}
	out, e := s.repo.List(filterUID, status)
	if e != nil {
		return nil, fmt.Errorf("VisitorPass list failed: %w", e)
	}
	for i := range out {
		out[i] = decorate(out[i])
	}
	return out, nil
}

// Approve 物业批准。冲突时整次拒绝并保持待审；重复 / 并发审批只有一次成功。
func (s *VisitorPassService) Approve(id, reviewerID uint, role string) (model.VisitorPass, error) {
	target, e := s.repo.ByID(id)
	if e != nil {
		return target, ErrVisitorNotFound
	}
	if target.Status != constants.VisitorStatusPending {
		// 已审批（含已批准/已拒绝/已撤销）：重复审批幂等返回当前状态，但不作为成功。
		s.logger.Info("visitor.approve.duplicate", "id", id, "status", target.Status, "role", role)
		return decorate(target), fmt.Errorf("%w: VisitorPass[id=%d] status=%s, current role=%s", ErrVisitorAlreadyReviewed, id, target.Status, role)
	}

	plate := target.NormalizedPlate
	mu := lockForPlate(plate)
	mu.Lock()
	defer mu.Unlock()

	now := time.Now().Truncate(time.Second)
	var result model.VisitorPass
	var conflicted bool
	txErr := s.repo.DB.Transaction(func(tx *gorm.DB) error {
		// 锁内复查：另一张申请可能刚取得同一车牌的重叠时段。
		// excludeID=id：本申请自身（可能已被并发请求抢先批准）不算冲突，交给下方条件更新判定。
		conflict, found, ce := s.repo.FindEffectiveOverlap(tx, plate, target.StartAt, target.EndAt, now, id)
		if ce != nil {
			return ce
		}
		if found {
			reason := fmt.Sprintf(constants.MessageVisitorConflictFmt, target.Plate, conflict.ID,
				conflict.StartAt.Format("2006-01-02 15:04"), conflict.EndAt.Format("2006-01-02 15:04"))
			if me := s.repo.MarkConflictInTx(tx, id, reviewerID, reason, now); me != nil {
				return me
			}
			result = target
			result.FailureReason = reason
			conflicted = true
			return nil // 提交失败原因，保持待审；冲突错误在事务提交后返回
		}
		rows, ae := s.repo.ApproveInTx(tx, id, reviewerID, now)
		if ae != nil {
			return ae
		}
		if rows == 0 {
			// 并发请求已先行审批。不在事务内查询（避免占用第二个连接），提交后统一回读。
			return fmt.Errorf("%w: VisitorPass[id=%d] approve raced, current role=%s", ErrVisitorAlreadyReviewed, id, role)
		}
		return nil
	})
	if txErr != nil {
		if errors.Is(txErr, ErrVisitorAlreadyReviewed) {
			cur, _ := s.repo.ByID(id)
			return decorate(cur), txErr
		}
		return target, fmt.Errorf("VisitorPass[id=%d] approve failed, current role=%s: %w", id, role, txErr)
	}
	if conflicted {
		s.logger.Warn("visitor.approve.conflict", "id", id, "plate", plate, "reviewer", reviewerID)
		// 重新读取，确保刷新后回读到已持久化的失败原因。
		refreshed, re := s.repo.ByID(id)
		if re == nil {
			result = refreshed
		}
		return decorate(result), fmt.Errorf("%w: %s", ErrVisitorConflict, result.FailureReason)
	}
	s.logger.Info("visitor.approve", "id", id, "plate", plate, "reviewer", reviewerID)
	got, ge := s.repo.ByID(id)
	return decorate(got), ge
}

// Reject 物业人工拒绝待审申请。
func (s *VisitorPassService) Reject(id, reviewerID uint, reason, role string) (model.VisitorPass, error) {
	target, e := s.repo.ByID(id)
	if e != nil {
		return target, ErrVisitorNotFound
	}
	if target.Status != constants.VisitorStatusPending {
		return decorate(target), fmt.Errorf("%w: VisitorPass[id=%d] status=%s, current role=%s", ErrVisitorAlreadyReviewed, id, target.Status, role)
	}
	mu := lockForPlate(target.NormalizedPlate)
	mu.Lock()
	defer mu.Unlock()

	now := time.Now().Truncate(time.Second)
	txErr := s.repo.DB.Transaction(func(tx *gorm.DB) error {
		rows, re := s.repo.RejectInTx(tx, id, reviewerID, reason, now)
		if re != nil {
			return re
		}
		if rows == 0 {
			return fmt.Errorf("%w: VisitorPass[id=%d] reject raced, current role=%s", ErrVisitorAlreadyReviewed, id, role)
		}
		return nil
	})
	if txErr != nil {
		if errors.Is(txErr, ErrVisitorAlreadyReviewed) {
			cur, _ := s.repo.ByID(id)
			return decorate(cur), txErr
		}
		return target, fmt.Errorf("VisitorPass[id=%d] reject failed, current role=%s: %w", id, role, txErr)
	}
	s.logger.Info("visitor.reject", "id", id, "reviewer", reviewerID)
	got, ge := s.repo.ByID(id)
	return decorate(got), ge
}

// Revoke 到访开始前撤销；撤销后时段立即释放。业主只能撤销自己的申请，物业可撤销任意申请。
func (s *VisitorPassService) Revoke(id, operatorID uint, role string) (model.VisitorPass, error) {
	target, e := s.repo.ByID(id)
	if e != nil {
		return target, ErrVisitorNotFound
	}
	if role == constants.UserRoleResident && target.UserID != operatorID {
		return target, fmt.Errorf("%w: VisitorPass[id=%d] revoke by user=%d owner=%d", ErrVisitorForbidden, id, operatorID, target.UserID)
	}
	if target.Status != constants.VisitorStatusApproved {
		return decorate(target), fmt.Errorf("%w: VisitorPass[id=%d] status=%s", ErrVisitorRevokeState, id, target.Status)
	}
	now := time.Now().Truncate(time.Second)
	if !target.StartAt.After(now) {
		return decorate(target), fmt.Errorf("%w: VisitorPass[id=%d] start_at=%s", ErrVisitorRevokeWindow, id, target.StartAt.Format(time.RFC3339))
	}
	mu := lockForPlate(target.NormalizedPlate)
	mu.Lock()
	defer mu.Unlock()

	var raced bool
	txErr := s.repo.DB.Transaction(func(tx *gorm.DB) error {
		rows, re := s.repo.RevokeInTx(tx, id, operatorID, now)
		if re != nil {
			return re
		}
		if rows == 0 {
			// 不在事务内回读：提交后根据最新行区分“状态已变”与“到访已开始”。
			raced = true
			return fmt.Errorf("%w: VisitorPass[id=%d] revoke raced", ErrVisitorRevokeWindow, id)
		}
		return nil
	})
	if txErr != nil {
		if raced {
			cur, ce := s.repo.ByID(id)
			if ce == nil && cur.Status != constants.VisitorStatusApproved {
				return decorate(cur), fmt.Errorf("%w: VisitorPass[id=%d] status=%s", ErrVisitorRevokeState, id, cur.Status)
			}
			return decorate(cur), txErr
		}
		return target, txErr
	}
	s.logger.Info("visitor.revoke", "id", id, "operator", operatorID, "role", role)
	got, ge := s.repo.ByID(id)
	return decorate(got), ge
}

func (s *VisitorPassService) PendingCount() (int64, error) {
	return s.repo.CountByStatus(constants.VisitorStatusPending)
}
