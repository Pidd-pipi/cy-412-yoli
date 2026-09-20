package router

import (
	"github.com/gin-gonic/gin"
	"github.com/smartestate/smartestate/internal/handler"
	"github.com/smartestate/smartestate/internal/middleware"
)

func RegisterVisitorPasses(g *gin.RouterGroup, sv Services, h *handler.Handler) {
	x := handler.NewVisitorPassHandler(sv.VisitorPasses, h)
	g.GET("/visitor-passes", x.List)
	g.POST("/visitor-passes", middleware.OperationLog(sv.Logs, "visitor_pass.create"), x.Create)
	g.POST("/visitor-passes/:id/approve", middleware.RequirePermission(sv.Permissions, "visitor:manage"), middleware.OperationLog(sv.Logs, "visitor_pass.approve"), x.Approve)
	g.POST("/visitor-passes/:id/revoke", middleware.OperationLog(sv.Logs, "visitor_pass.revoke"), x.Revoke)
}
