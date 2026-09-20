package model

import "time"

type VisitorPass struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	UserID     uint       `gorm:"index" json:"user_id"`
	User       User       `json:"user"`
	PlateNo    string     `gorm:"index;size:20" json:"plate_no"`
	Building   string     `json:"building"`
	Unit       string     `json:"unit"`
	Room       string     `json:"room"`
	VisitStart time.Time  `json:"visit_start"`
	VisitEnd   time.Time  `json:"visit_end"`
	Status     string     `gorm:"index;size:20" json:"status"`
	FailReason string     `json:"fail_reason"`
	ApprovedBy *uint      `json:"approved_by"`
	ApprovedAt *time.Time `json:"approved_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}
