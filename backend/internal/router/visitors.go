package router

import (
	"github.com/gin-gonic/gin"
	"github.com/smartestate/smartestate/internal/handler"
	"github.com/smartestate/smartestate/internal/middleware"
)

func RegisterVisitorPasses(g *gin.RouterGroup, sv Services, h *handler.Handler) {
	x := handler.NewVisitorPassHandler(sv.VisitorPasses, h)
	g.GET("/visitor-passes", middleware.OperationLog(sv.Logs, "visitor.list"), x.List)
	g.POST("/visitor-passes", middleware.OperationLog(sv.Logs, "visitor.create"), x.Create)
	g.POST("/visitor-passes/:id/approve", middleware.RequirePermission(sv.Permissions, "visitor:approve"), middleware.OperationLog(sv.Logs, "visitor.approve"), x.Approve)
	g.POST("/visitor-passes/:id/reject", middleware.RequirePermission(sv.Permissions, "visitor:approve"), middleware.OperationLog(sv.Logs, "visitor.reject"), x.Reject)
	g.POST("/visitor-passes/:id/revoke", middleware.OperationLog(sv.Logs, "visitor.revoke"), x.Revoke)
}
