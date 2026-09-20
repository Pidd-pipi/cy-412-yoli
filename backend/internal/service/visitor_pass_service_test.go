package service

import (
	"errors"
	"github.com/smartestate/smartestate/internal/constants"
	"github.com/smartestate/smartestate/internal/model"
	"github.com/smartestate/smartestate/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"
)

func newVisitorPassService(t *testing.T) (*VisitorPassService, model.User) {
	t.Helper()
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if e := db.AutoMigrate(&model.User{}, &model.VisitorPass{}); e != nil {
		t.Fatal(e)
	}
	u := model.User{Phone: "13800000001", Role: "resident"}
	if e := db.Create(&u).Error; e != nil {
		t.Fatal(e)
	}
	return NewVisitorPassService(repository.NewVisitorPassRepository(db), slog.New(slog.NewTextHandler(io.Discard, nil))), u
}

func TestVisitorPassClosedLoop(t *testing.T) {
	s, u := newVisitorPassService(t)
	start := time.Now().Add(2 * time.Hour)
	end := start.Add(2 * time.Hour)
	a, e := s.Create(u.ID, "沪a12345", "1栋", "2单元", "802", start, end, "resident")
	if e != nil || a.Status != constants.VisitorPassStatusPending || a.PlateNo != "沪A12345" {
		t.Fatalf("create got %+v err %v", a, e)
	}
	if a, e = s.Approve(a.ID, 2, "staff"); e != nil || a.Status != constants.VisitorPassStatusApproved {
		t.Fatalf("approve got %+v err %v", a, e)
	}
	if _, e = s.Approve(a.ID, 2, "staff"); !errors.Is(e, ErrVisitorPassNotPending) {
		t.Fatalf("repeated approve should fail once, got %v", e)
	}
	b, e := s.Create(u.ID, "沪A12345", "1栋", "2单元", "802", start.Add(time.Hour), end.Add(time.Hour), "resident")
	if e != nil {
		t.Fatalf("create overlapping application err %v", e)
	}
	if _, e = s.Approve(b.ID, 2, "staff"); !errors.Is(e, ErrVisitorPassConflict) {
		t.Fatalf("conflicting approve should be rejected, got %v", e)
	}
	stayed, _ := s.List(0, "staff", constants.VisitorPassStatusPending)
	if len(stayed) != 1 || stayed[0].ID != b.ID || stayed[0].FailReason == "" {
		t.Fatalf("conflict must keep pending with fail reason, got %+v", stayed)
	}
	if _, e = s.Revoke(a.ID, u.ID, "resident"); e != nil {
		t.Fatalf("revoke before visit start err %v", e)
	}
	if b, e = s.Approve(b.ID, 2, "staff"); e != nil || b.Status != constants.VisitorPassStatusApproved || b.FailReason != "" {
		t.Fatalf("slot released after revoke, approve got %+v err %v", b, e)
	}
}

func TestVisitorPassExpiryReleasesSlot(t *testing.T) {
	s, u := newVisitorPassService(t)
	start := time.Now().Add(-3 * time.Hour)
	end := time.Now().Add(-time.Hour)
	old := model.VisitorPass{UserID: u.ID, PlateNo: "沪B66666", Building: "1栋", Room: "802", VisitStart: start, VisitEnd: end, Status: constants.VisitorPassStatusApproved}
	if e := s.repo.Create(&old); e != nil {
		t.Fatal(e)
	}
	c, e := s.Create(u.ID, "沪B66666", "1栋", "", "802", time.Now().Add(time.Hour), time.Now().Add(2*time.Hour), "resident")
	if e != nil {
		t.Fatal(e)
	}
	if c, e = s.Approve(c.ID, 2, "staff"); e != nil || c.Status != constants.VisitorPassStatusApproved {
		t.Fatalf("expired pass must release slot, got %+v err %v", c, e)
	}
	list, _ := s.List(0, "staff", constants.VisitorPassStatusExpired)
	if len(list) != 1 || list[0].ID != old.ID {
		t.Fatalf("stale pass should be lazily expired, got %+v", list)
	}
}

func TestVisitorPassConcurrentApproveOnlyOnce(t *testing.T) {
	s, u := newVisitorPassService(t)
	a, e := s.Create(u.ID, "沪C00001", "1栋", "", "802", time.Now().Add(time.Hour), time.Now().Add(2*time.Hour), "resident")
	if e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	succeeded := 0
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Approve(a.ID, 2, "staff"); err == nil {
				mu.Lock()
				succeeded++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if succeeded != 1 {
		t.Fatalf("concurrent approvals must succeed exactly once, got %d", succeeded)
	}
}

func TestVisitorPassRevokeWindow(t *testing.T) {
	s, u := newVisitorPassService(t)
	started, e := s.Create(u.ID, "沪D00002", "1栋", "", "802", time.Now().Add(-time.Hour), time.Now().Add(time.Hour), "resident")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Revoke(started.ID, u.ID, "resident"); e == nil {
		t.Fatal("revoke after visit start must fail")
	}
	other, e := s.Create(u.ID, "沪D00003", "1栋", "", "802", time.Now().Add(time.Hour), time.Now().Add(2*time.Hour), "resident")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Revoke(other.ID, 999, "resident"); e == nil {
		t.Fatal("resident cannot revoke others' pass")
	}
	if r, e := s.Revoke(other.ID, 2, "staff"); e != nil || r.Status != constants.VisitorPassStatusRevoked {
		t.Fatalf("staff revoke got %+v err %v", r, e)
	}
}
