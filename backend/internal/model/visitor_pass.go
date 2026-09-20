package model

import "time"

// VisitorPass 访客通行证：登记后生成待审申请，物业批准后成为有效通行证。
type VisitorPass struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	UserID          uint       `gorm:"index" json:"user_id"`
	User            User       `json:"user"`
	Plate           string     `gorm:"size:16" json:"plate"`
	NormalizedPlate string     `gorm:"size:16;index:idx_visitor_plate_time,priority:1" json:"normalized_plate"`
	VisitorName     string     `gorm:"size:30" json:"visitor_name"`
	Building        string     `gorm:"size:30" json:"building"`
	Room            string     `gorm:"size:30" json:"room"`
	StartAt         time.Time  `gorm:"index:idx_visitor_plate_time,priority:2" json:"start_at"`
	EndAt           time.Time  `json:"end_at"`
	Status          string     `gorm:"index;size:20" json:"status"`
	ReviewerID      *uint      `json:"reviewer_id"`
	Reviewer        *User      `gorm:"foreignKey:ReviewerID" json:"reviewer,omitempty"`
	ReviewedAt      *time.Time `json:"reviewed_at"`
	FailureReason   string     `gorm:"size:200" json:"failure_reason"`
	RevokedByID     *uint      `json:"revoked_by_id"`
	RevokedAt       *time.Time `json:"revoked_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`

	// EffectiveState 仅在序列化时由 service 计算（approved -> upcoming/active/expired），不持久化。
	EffectiveState string `gorm:"-" json:"effective_state,omitempty"`
}
