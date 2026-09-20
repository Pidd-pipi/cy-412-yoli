package handler

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/smartestate/smartestate/internal/constants"
	"github.com/smartestate/smartestate/internal/dto"
	"github.com/smartestate/smartestate/internal/service"
	"net/http"
	"strconv"
)

type VisitorPassHandler struct {
	Handler
	svc *service.VisitorPassService
}

func NewVisitorPassHandler(s *service.VisitorPassService, h *Handler) *VisitorPassHandler {
	return &VisitorPassHandler{Handler: *h, svc: s}
}
func (h *VisitorPassHandler) List(c *gin.Context) {
	v, e := h.svc.List(c.GetUint("userID"), c.GetString("role"), c.Query("status"))
	if e != nil {
		Fail(c, 400, constants.CodeBadRequest, e.Error())
		return
	}
	OK(c, v)
}
func (h *VisitorPassHandler) Create(c *gin.Context) {
	var r dto.CreateVisitorPassRequest
	if !Bind(c, &r, h.Validate) {
		return
	}
	v, e := h.svc.Create(c.GetUint("userID"), r.PlateNo, r.Building, r.Unit, r.Room, r.VisitStart, r.VisitEnd, c.GetString("role"))
	if e != nil {
		Fail(c, 400, constants.CodeBadRequest, e.Error())
		return
	}
	OK(c, v)
}
func (h *VisitorPassHandler) Approve(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	v, e := h.svc.Approve(uint(id), c.GetUint("userID"), c.GetString("role"))
	if e != nil {
		if errors.Is(e, service.ErrVisitorPassConflict) || errors.Is(e, service.ErrVisitorPassNotPending) {
			Fail(c, http.StatusConflict, constants.CodeConflict, e.Error())
			return
		}
		Fail(c, 400, constants.CodeBadRequest, e.Error())
		return
	}
	OK(c, v)
}
func (h *VisitorPassHandler) Revoke(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	v, e := h.svc.Revoke(uint(id), c.GetUint("userID"), c.GetString("role"))
	if e != nil {
		Fail(c, 400, constants.CodeBadRequest, e.Error())
		return
	}
	OK(c, v)
}
