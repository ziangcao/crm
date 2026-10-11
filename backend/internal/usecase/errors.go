package usecase

import "cordialcrm/backend/internal/entity"

var (
	ErrUnauthorized = entity.NewBusinessError("unauthorized", "请先登录")
	ErrForbidden    = entity.NewBusinessError("forbidden", "没有操作权限")
	ErrNotFound     = entity.NewBusinessError("not_found", "记录不存在")
	ErrConflict     = entity.NewBusinessError("conflict", "记录已被其他人更新，请刷新后重试")
	ErrValidation   = entity.NewBusinessError("validation", "请求参数无效")
)
