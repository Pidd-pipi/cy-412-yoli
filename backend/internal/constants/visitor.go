package constants

// VisitorPassStatus 访客通行证申请状态。
const (
	VisitorStatusPending  = "pending"  // 待审
	VisitorStatusApproved = "approved" // 已批准（有效通行证，过期前有效）
	VisitorStatusRejected = "rejected" // 审批未通过（冲突或人工拒绝），保持/退回待审语义由前端提示
	VisitorStatusRevoked  = "revoked"  // 到访开始前被撤销
)

// VisitorEffectiveState 是审批通过后根据当前时间计算的有效态（不入库）。
const (
	VisitorStateUpcoming = "upcoming" // 未到到访开始时间
	VisitorStateActive   = "active"   // 到访时段内
	VisitorStateExpired  = "expired"  // 已过期，时段已释放
)

var ValidVisitorStatuses = map[string]bool{
	VisitorStatusPending:  true,
	VisitorStatusApproved: true,
	VisitorStatusRejected: true,
	VisitorStatusRevoked:  true,
}

// VisitorPlateMinLen/VisitorPlateMaxLen 车牌长度边界（含新能源 8 位）。
const (
	VisitorPlateMinLen  = 5
	VisitorPlateMaxLen  = 16
	VisitorReasonMaxLen = 200
)
