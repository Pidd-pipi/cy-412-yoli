package service

import (
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/smartestate/smartestate/internal/constants"
	"github.com/smartestate/smartestate/internal/model"
	"github.com/smartestate/smartestate/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newVisitorTestDB(t *testing.T) (*gorm.DB, *VisitorPassService, model.User) {
	t.Helper()
	// 共享内存 DSN + 单连接池，使并发 goroutine 访问同一份数据并由 SQLite 事务串行化。
	db, e := gorm.Open(sqlite.Open("file::memory:?cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if e != nil {
		t.Fatalf("open db: %v", e)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if e = db.Exec("DROP TABLE IF EXISTS visitor_passes; DROP TABLE IF EXISTS users;").Error; e != nil {
		t.Fatalf("reset db: %v", e)
	}
	if e = db.AutoMigrate(&model.User{}, &model.VisitorPass{}); e != nil {
		t.Fatalf("migrate: %v", e)
	}
	u := model.User{Phone: "13800000001", Nickname: "张业主", Role: "resident"}
	if e = db.Create(&u).Error; e != nil {
		t.Fatalf("create user: %v", e)
	}
	s := NewVisitorPassService(repository.NewVisitorPassRepository(db), slog.New(slog.NewTextHandler(io.Discard, nil)))
	return db, s, u
}

func iso(offset, dur time.Duration) (string, string) {
	st := time.Now().Add(offset).Truncate(time.Minute)
	return st.Format(time.RFC3339), st.Add(dur).Format(time.RFC3339)
}

func TestVisitorCreateAndList(t *testing.T) {
	_, s, u := newVisitorTestDB(t)
	start, end := iso(2*time.Hour, 2*time.Hour)
	v, e := s.Create(u.ID, "粤b 8a888", "李师傅", "1栋", "802", start, end)
	if e != nil {
		t.Fatalf("create: %v", e)
	}
	if v.Status != constants.VisitorStatusPending {
		t.Fatalf("new pass must be pending, got %s", v.Status)
	}
	if v.NormalizedPlate != "粤B8A888" {
		t.Fatalf("normalized plate = %q", v.NormalizedPlate)
	}
	// 业主只能回读自己的申请
	got, e := s.List(u.ID, "resident", "")
	if e != nil || len(got) != 1 {
		t.Fatalf("resident list len=%d err=%v", len(got), e)
	}
	staffList, _ := s.List(0, "staff", "")
	if len(staffList) != 1 {
		t.Fatalf("staff should see all passes, got %d", len(staffList))
	}
}

func TestVisitorCreateInvalidWindow(t *testing.T) {
	_, s, u := newVisitorTestDB(t)
	past := time.Now().Add(-2 * time.Hour).Truncate(time.Minute)
	cases := []struct{ start, end string }{
		{past.Format(time.RFC3339), past.Add(time.Hour).Format(time.RFC3339)},                        // 开始时间在过去
		{past.Add(3 * time.Hour).Format(time.RFC3339), past.Add(2 * time.Hour).Format(time.RFC3339)}, // 结束早于开始
		{"not-a-time", "not-a-time"},
	}
	for _, c := range cases {
		if _, e := s.Create(u.ID, "京A12345", "", "1栋", "801", c.start, c.end); !errors.Is(e, ErrVisitorInvalidTime) {
			t.Fatalf("want ErrVisitorInvalidTime, got %v", e)
		}
	}
}

func TestVisitorApproveAndDuplicate(t *testing.T) {
	_, s, u := newVisitorTestDB(t)
	start, end := iso(3*time.Hour, time.Hour)
	v, _ := s.Create(u.ID, "沪A11111", "", "2栋", "301", start, end)

	approved, e := s.Approve(v.ID, 99, "staff")
	if e != nil || approved.Status != constants.VisitorStatusApproved {
		t.Fatalf("approve status=%s err=%v", approved.Status, e)
	}
	if approved.EffectiveState != constants.VisitorStateUpcoming {
		t.Fatalf("effective state = %q", approved.EffectiveState)
	}
	// 重复审批只能成功一次：第二次必须失败
	if _, e = s.Approve(v.ID, 99, "staff"); !errors.Is(e, ErrVisitorAlreadyReviewed) {
		t.Fatalf("duplicate approve want ErrVisitorAlreadyReviewed, got %v", e)
	}
}

func TestVisitorOverlapConflictKeepsPending(t *testing.T) {
	db, s, u := newVisitorTestDB(t)
	base := time.Now().Add(24 * time.Hour).Truncate(time.Minute)
	mk := func(st, en time.Time) model.VisitorPass {
		p := model.VisitorPass{UserID: u.ID, Plate: "粤B99999", NormalizedPlate: "粤B99999", Building: "1栋", Room: "802", StartAt: st, EndAt: en, Status: constants.VisitorStatusPending}
		if e := db.Create(&p).Error; e != nil {
			t.Fatalf("seed pass: %v", e)
		}
		return p
	}
	a := mk(base, base.Add(2*time.Hour))
	if _, e := s.Approve(a.ID, 99, "staff"); e != nil {
		t.Fatalf("first approve: %v", e)
	}
	// 完全重叠
	b := mk(base.Add(30*time.Minute), base.Add(90*time.Minute))
	_, e := s.Approve(b.ID, 99, "staff")
	if !errors.Is(e, ErrVisitorConflict) {
		t.Fatalf("overlap want ErrVisitorConflict, got %v", e)
	}
	after, _ := s.repoByIDForTest(t, b.ID)
	if after.Status != constants.VisitorStatusPending {
		t.Fatalf("conflict must keep pending, got %s", after.Status)
	}
	if after.FailureReason == "" {
		t.Fatalf("conflict must record failure reason")
	}
	// 首尾相接（b.start == a.end）不冲突
	c := mk(base.Add(2*time.Hour), base.Add(3*time.Hour))
	if _, e = s.Approve(c.ID, 99, "staff"); e != nil {
		t.Fatalf("back-to-back windows should not conflict: %v", e)
	}
	// 冲突保持待审后，第一张撤销释放时段，再次审批应成功
	if _, e = s.Revoke(a.ID, u.ID, "resident"); e != nil {
		t.Fatalf("revoke: %v", e)
	}
	if _, e = s.Approve(b.ID, 99, "staff"); e != nil {
		t.Fatalf("re-approve after revoke should succeed: %v", e)
	}
}

func TestVisitorExpiredPassReleasesSlot(t *testing.T) {
	db, s, u := newVisitorTestDB(t)
	now := time.Now().Add(-time.Minute)
	expired := model.VisitorPass{UserID: u.ID, Plate: "粤A77777", NormalizedPlate: "粤A77777", Building: "1栋", Room: "802", StartAt: now.Add(-2 * time.Hour), EndAt: now.Add(-time.Hour), Status: constants.VisitorStatusApproved}
	if e := db.Create(&expired).Error; e != nil {
		t.Fatalf("seed expired: %v", e)
	}
	start, end := iso(2*time.Hour, time.Hour)
	next, e := s.Create(u.ID, "粤a77777", "", "1栋", "802", start, end)
	if e != nil {
		t.Fatalf("create: %v", e)
	}
	if _, e = s.Approve(next.ID, 99, "staff"); e != nil {
		t.Fatalf("expired pass should not block new approval: %v", e)
	}
}

func TestVisitorRevokeRules(t *testing.T) {
	db, s, u := newVisitorTestDB(t)
	// 过期通行证不可撤销
	st := time.Now().Add(-3 * time.Hour).Truncate(time.Minute)
	old := model.VisitorPass{UserID: u.ID, Plate: "粤C55555", NormalizedPlate: "粤C55555", Building: "1栋", Room: "802", StartAt: st, EndAt: st.Add(time.Hour), Status: constants.VisitorStatusApproved}
	db.Create(&old)
	if _, e := s.Revoke(old.ID, u.ID, "resident"); !errors.Is(e, ErrVisitorRevokeWindow) {
		t.Fatalf("expired revoke want ErrVisitorRevokeWindow, got %v", e)
	}
	// 开始前撤销成功
	start, end := iso(4*time.Hour, time.Hour)
	v, _ := s.Create(u.ID, "粤C55555", "", "1栋", "802", start, end)
	s.Approve(v.ID, 99, "staff")
	got, e := s.Revoke(v.ID, u.ID, "resident")
	if e != nil || got.Status != constants.VisitorStatusRevoked {
		t.Fatalf("revoke status=%s err=%v", got.Status, e)
	}
	if got.RevokedAt == nil {
		t.Fatalf("revoked_at must be set")
	}
	// 重复撤销
	if _, e = s.Revoke(v.ID, u.ID, "resident"); !errors.Is(e, ErrVisitorRevokeState) {
		t.Fatalf("double revoke want ErrVisitorRevokeState, got %v", e)
	}
	// 业主不能撤销他人申请
	other := model.User{Phone: "13800000009", Role: "resident"}
	db.Create(&other)
	start2, end2 := iso(5*time.Hour, time.Hour)
	v2, _ := s.Create(other.ID, "粤C66666", "", "1栋", "803", start2, end2)
	s.Approve(v2.ID, 99, "staff")
	if _, e = s.Revoke(v2.ID, u.ID, "resident"); !errors.Is(e, ErrVisitorForbidden) {
		t.Fatalf("cross-user revoke want ErrVisitorForbidden, got %v", e)
	}
}

func TestVisitorRejectFlow(t *testing.T) {
	_, s, u := newVisitorTestDB(t)
	start, end := iso(6*time.Hour, time.Hour)
	v, _ := s.Create(u.ID, "粤D22222", "", "1栋", "802", start, end)
	got, e := s.Reject(v.ID, 99, "楼栋房号信息不完整", "staff")
	if e != nil || got.Status != constants.VisitorStatusRejected || got.FailureReason != "楼栋房号信息不完整" {
		t.Fatalf("reject status=%s reason=%q err=%v", got.Status, got.FailureReason, e)
	}
	if _, e = s.Reject(v.ID, 99, "再次拒绝", "staff"); !errors.Is(e, ErrVisitorAlreadyReviewed) {
		t.Fatalf("reject twice want ErrVisitorAlreadyReviewed, got %v", e)
	}
}

// helper kept as a method-free lookup to avoid adding test-only production API.
func (s *VisitorPassService) repoByIDForTest(t *testing.T, id uint) (model.VisitorPass, error) {
	t.Helper()
	return s.repo.ByID(id)
}

func TestVisitorConcurrentApproveOnlyOne(t *testing.T) {
	db, s, u := newVisitorTestDB(t)
	base := time.Now().Add(48 * time.Hour).Truncate(time.Minute)
	mkPending := func(st, en time.Time) uint {
		p := model.VisitorPass{UserID: u.ID, Plate: "粤Z12321", NormalizedPlate: "粤Z12321", Building: "1栋", Room: "802", StartAt: st, EndAt: en, Status: constants.VisitorStatusPending}
		if e := db.Create(&p).Error; e != nil {
			t.Fatalf("seed: %v", e)
		}
		return p.ID
	}
	id1 := mkPending(base, base.Add(2*time.Hour))
	id2 := mkPending(base.Add(30*time.Minute), base.Add(3*time.Hour))

	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make([]error, 2)
	approve := func(idx, pid uint) {
		defer wg.Done()
		<-start
		_, results[idx] = s.Approve(pid, 99, "staff")
	}
	wg.Add(2)
	go approve(0, id1)
	go approve(1, id2)
	close(start)
	wg.Wait()

	succ, conflict := 0, 0
	for _, e := range results {
		switch {
		case e == nil:
			succ++
		case errors.Is(e, ErrVisitorConflict):
			conflict++
		default:
			t.Fatalf("unexpected concurrent result: %v", e)
		}
	}
	if succ != 1 || conflict != 1 {
		t.Fatalf("concurrent approve want 1 success + 1 conflict, got %d success / %d conflict", succ, conflict)
	}
	var approved int64
	db.Model(&model.VisitorPass{}).Where("normalized_plate = ? AND status = ?", "粤Z12321", "approved").Count(&approved)
	if approved != 1 {
		t.Fatalf("exactly one effective pass allowed, got %d", approved)
	}
}

func TestVisitorConcurrentDuplicateApproveOnlyOne(t *testing.T) {
	db, s, u := newVisitorTestDB(t)
	base := time.Now().Add(72 * time.Hour).Truncate(time.Minute)
	p := model.VisitorPass{UserID: u.ID, Plate: "粤Z55555", NormalizedPlate: "粤Z55555", Building: "1栋", Room: "802", StartAt: base, EndAt: base.Add(time.Hour), Status: constants.VisitorStatusPending}
	db.Create(&p)

	const n = 8
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make([]error, n)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			<-start
			_, errs[idx] = s.Approve(p.ID, 99, "staff")
		}(i)
	}
	close(start)
	wg.Wait()
	succ := 0
	for _, e := range errs {
		if e == nil {
			succ++
		} else if !errors.Is(e, ErrVisitorAlreadyReviewed) {
			t.Fatalf("unexpected duplicate result: %v", e)
		}
	}
	if succ != 1 {
		t.Fatalf("duplicate concurrent approve must succeed once, got %d", succ)
	}
}
