package handler

import (
	"errors"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/smartestate/smartestate/internal/constants"
	"github.com/smartestate/smartestate/internal/dto"
	"github.com/smartestate/smartestate/internal/service"
)

type VisitorPassHandler struct {
	Handler
	svc *service.VisitorPassService
}

func NewVisitorPassHandler(s *service.VisitorPassService, h *Handler) *VisitorPassHandler {
	return &VisitorPassHandler{Handler: *h, svc: s}
}

// failVisitor 把 service 哨兵错误映射为统一 JSON 响应。
func failVisitor(c *gin.Context, e error) {
	switch {
	case errors.Is(e, service.ErrVisitorNotFound):
		Fail(c, 404, constants.CodeNotFound, e.Error())
	case errors.Is(e, service.ErrVisitorInvalidTime), errors.Is(e, service.ErrVisitorForbidden):
		Fail(c, 400, constants.CodeBadRequest, e.Error())
	case errors.Is(e, service.ErrVisitorConflict):
		// 整次拒绝、保持待审：按业务失败返回 409，消息即页面展示的失败原因。
		Fail(c, 409, constants.CodeConflict, e.Error())
	case errors.Is(e, service.ErrVisitorAlreadyReviewed), errors.Is(e, service.ErrVisitorRevokeWindow), errors.Is(e, service.ErrVisitorRevokeState):
		Fail(c, 409, constants.CodeConflict, e.Error())
	default:
		Fail(c, 500, constants.CodeInternal, e.Error())
	}
}

func (h *VisitorPassHandler) List(c *gin.Context) {
	v, e := h.svc.List(c.GetUint("userID"), c.GetString("role"), c.Query("status"))
	if e != nil {
		Fail(c, 500, constants.CodeInternal, e.Error())
		return
	}
	OK(c, v)
}

func (h *VisitorPassHandler) Create(c *gin.Context) {
	var r dto.CreateVisitorPassRequest
	if !Bind(c, &r, h.Validate) {
		return
	}
	if _, perr := time.Parse(time.RFC3339, r.StartAt); perr != nil {
		Fail(c, 400, constants.CodeBadRequest, constants.MessageVisitorTimeWindow)
		return
	}
	if _, perr := time.Parse(time.RFC3339, r.EndAt); perr != nil {
		Fail(c, 400, constants.CodeBadRequest, constants.MessageVisitorTimeWindow)
		return
	}
	v, e := h.svc.Create(c.GetUint("userID"), r.Plate, r.VisitorName, r.Building, r.Room, r.StartAt, r.EndAt)
	if e != nil {
		failVisitor(c, e)
		return
	}
	OK(c, v)
}

func (h *VisitorPassHandler) Approve(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	v, e := h.svc.Approve(uint(id), c.GetUint("userID"), c.GetString("role"))
	if e != nil {
		failVisitor(c, e)
		return
	}
	OK(c, v)
}

func (h *VisitorPassHandler) Reject(c *gin.Context) {
	var r dto.RejectVisitorPassRequest
	if !Bind(c, &r, h.Validate) {
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	v, e := h.svc.Reject(uint(id), c.GetUint("userID"), r.Reason, c.GetString("role"))
	if e != nil {
		failVisitor(c, e)
		return
	}
	OK(c, v)
}

func (h *VisitorPassHandler) Revoke(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	v, e := h.svc.Revoke(uint(id), c.GetUint("userID"), c.GetString("role"))
	if e != nil {
		failVisitor(c, e)
		return
	}
	OK(c, v)
}
