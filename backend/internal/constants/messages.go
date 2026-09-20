package constants

const (
	MessageOK             = "ok"
	MessageUnauthorized   = "登录已失效"
	MessageForbidden      = "无权限执行此操作"
	MessageValidation     = "请求参数不合法"
	MessageNotFound       = "资源不存在"
	MessagePaymentSuccess = "支付宝沙箱支付成功"
	MessageRepairCreated  = "报修工单已提交"

	MessageVisitorCreated       = "访客通行申请已提交，等待物业审批"
	MessageVisitorApproved      = "通行证已批准"
	MessageVisitorRejected      = "申请已拒绝"
	MessageVisitorRevoked       = "通行证已撤销，到访时段已释放"
	MessageVisitorConflictFmt   = "同一车牌 %s 在重叠时段已有有效通行证（申请单 #%d：%s - %s），本次审批整次拒绝，申请保持待审"
	MessageVisitorAlreadyReview = "该申请已审批，重复审批未生效"
	MessageVisitorReviewWindow  = "仅待审申请可以审批"
	MessageVisitorRevokeWindow  = "到访开始后通行证不可撤销"
	MessageVisitorRevokeState   = "仅未过期的有效通行证可以撤销"
	MessageVisitorTimeWindow    = "到访结束时间必须晚于开始时间，且开始时间需要在当前时间之后"
)
