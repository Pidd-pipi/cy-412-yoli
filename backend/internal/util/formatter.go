package util

import (
	"fmt"
	"github.com/smartestate/smartestate/internal/constants"
	"time"
)

func Date(v time.Time) string { return v.Format("2006-01-02 15:04") }
func Money(v float64) string  { return fmt.Sprintf("¥%.2f", v) }
func StatusText(v string) string {
	m := map[string]string{constants.RepairStatusPending: "待受理", constants.RepairStatusAssigned: "已分派", constants.RepairStatusProcessing: "处理中", constants.RepairStatusDone: "已完成", constants.RepairStatusClosed: "已关闭"}
	return m[v]
}
func RoleText(v string) string {
	m := map[string]string{constants.UserRoleResident: "业主", constants.UserRoleStaff: "物业人员", constants.UserRoleAdmin: "管理员"}
	return m[v]
}

// VisitorStatusText 访客申请状态文本（页面状态徽标与日志共用）。
func VisitorStatusText(v string) string {
	m := map[string]string{
		constants.VisitorStatusPending:  "待审",
		constants.VisitorStatusApproved: "已批准",
		constants.VisitorStatusRejected: "已拒绝",
		constants.VisitorStatusRevoked:  "已撤销",
	}
	return m[v]
}

// VisitorEffectiveStateText 有效通行证的时段态文本。
func VisitorEffectiveStateText(v string) string {
	m := map[string]string{
		constants.VisitorStateUpcoming: "未生效",
		constants.VisitorStateActive:   "通行中",
		constants.VisitorStateExpired:  "已过期",
	}
	return m[v]
}
