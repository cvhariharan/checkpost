package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"github.com/cvhariharan/checkpost/internal/core"
	"github.com/cvhariharan/checkpost/internal/models"
	"github.com/labstack/echo/v4"
)

// HandleListRoles returns the fixed built-in role catalog + permission matrix.
func (h *Handler) HandleListRoles(c echo.Context) error {
	return c.JSON(http.StatusOK, RolesResponse{
		Roles:   h.c.Roles(),
		Catalog: h.c.PermissionCatalog(),
	})
}

func (h *Handler) HandleListUsers(c echo.Context) error {
	var req PaginateRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	if req.Page > 0 {
		req.Page -= 1
	}
	if req.Count == 0 {
		req.Count = CountPerPage
	}

	page, err := h.c.ListUsers(c.Request().Context(), models.PageRequest{Page: req.Page, Count: req.Count})
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not list users", err, nil)
	}
	return c.JSON(http.StatusOK, ListUsersResponse{
		Users:      page.Items,
		TotalCount: page.TotalCount,
		PageCount:  page.PageCount,
	})
}

func (h *Handler) HandleCreateUser(c echo.Context) error {
	var req CreateUserRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	user, err := h.c.CreateUser(c.Request().Context(), models.CreateUser{
		Name:  req.Name,
		Email: req.Email,
	})
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not create user", err, nil)
	}
	return c.JSON(http.StatusCreated, user)
}

func (h *Handler) HandleUpdateUser(c echo.Context) error {
	var req UpdateUserRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	user, err := h.c.UpdateUser(c.Request().Context(), models.UpdateUser{
		UUID:     req.ID,
		Name:     req.Name,
		Email:    req.Email,
		Disabled: req.Disabled,
	})
	if err != nil {
		if errors.Is(err, core.ErrSystemUser) {
			return wrapError(http.StatusForbidden, "system user cannot be modified", err, nil)
		}
		return wrapError(http.StatusInternalServerError, "could not update user", err, nil)
	}
	return c.JSON(http.StatusOK, user)
}

func (h *Handler) HandleDeleteUser(c echo.Context) error {
	var req GetRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	if err := h.c.DeleteUser(c.Request().Context(), req.ID); err != nil {
		if errors.Is(err, core.ErrSystemUser) {
			return wrapError(http.StatusForbidden, "system user cannot be deleted", err, nil)
		}
		if errors.Is(err, sql.ErrNoRows) {
			return wrapError(http.StatusNotFound, "user not found", err, nil)
		}
		return wrapError(http.StatusInternalServerError, "could not delete user", err, nil)
	}
	return c.NoContent(http.StatusOK)
}

func (h *Handler) HandleListUserGroups(c echo.Context) error {
	var req PaginateRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	if req.Page > 0 {
		req.Page -= 1
	}
	if req.Count == 0 {
		req.Count = CountPerPage
	}

	page, err := h.c.ListUserGroups(c.Request().Context(), models.PageRequest{Page: req.Page, Count: req.Count})
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not list user groups", err, nil)
	}
	return c.JSON(http.StatusOK, ListUserGroupsResponse{
		UserGroups: page.Items,
		TotalCount: page.TotalCount,
		PageCount:  page.PageCount,
	})
}

func (h *Handler) HandleCreateUserGroup(c echo.Context) error {
	var req CreateUserGroupRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	group, err := h.c.CreateUserGroup(c.Request().Context(), models.CreateUserGroup{
		Name:           req.Name,
		Description:    req.Description,
		OIDCClaimValue: req.OIDCClaimValue,
	})
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not create user group", err, nil)
	}
	return c.JSON(http.StatusCreated, group)
}

func (h *Handler) HandleGetUserGroup(c echo.Context) error {
	var req GetRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	group, err := h.c.GetUserGroup(c.Request().Context(), req.ID)
	if err != nil {
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("could not get user group %s", req.ID), err, nil)
	}
	return c.JSON(http.StatusOK, group)
}

func (h *Handler) HandleUpdateUserGroup(c echo.Context) error {
	var req UpdateUserGroupRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	group, err := h.c.UpdateUserGroup(c.Request().Context(), models.UpdateUserGroup{
		UUID:           req.ID,
		Name:           req.Name,
		Description:    req.Description,
		OIDCClaimValue: req.OIDCClaimValue,
	})
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not update user group", err, nil)
	}
	return c.JSON(http.StatusOK, group)
}

func (h *Handler) HandleDeleteUserGroup(c echo.Context) error {
	var req GetRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	if err := h.c.DeleteUserGroup(c.Request().Context(), req.ID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return wrapError(http.StatusNotFound, "user group not found", err, nil)
		}
		return wrapError(http.StatusInternalServerError, "could not delete user group", err, nil)
	}
	return c.NoContent(http.StatusOK)
}

func (h *Handler) HandleListUserGroupMembers(c echo.Context) error {
	var req GetRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	members, err := h.c.ListUserGroupMembers(c.Request().Context(), req.ID)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not list members", err, nil)
	}
	return c.JSON(http.StatusOK, UserGroupMembersResponse{Members: members})
}

func (h *Handler) HandleAddUserGroupMember(c echo.Context) error {
	var req AddUserGroupMemberRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	if err := h.c.AddUserGroupMember(c.Request().Context(), req.ID, req.UserID); err != nil {
		return wrapError(http.StatusInternalServerError, "could not add member", err, nil)
	}
	return c.NoContent(http.StatusOK)
}

func (h *Handler) HandleRemoveUserGroupMember(c echo.Context) error {
	var req RemoveUserGroupMemberRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	if err := h.c.RemoveUserGroupMember(c.Request().Context(), req.ID, req.UserID); err != nil {
		return wrapError(http.StatusInternalServerError, "could not remove member", err, nil)
	}
	return c.NoContent(http.StatusOK)
}

func (h *Handler) HandleListRoleBindings(c echo.Context) error {
	var req ListRoleBindingsRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	bindings, err := h.c.ListBindingsForSubject(c.Request().Context(), req.SubjectType, req.SubjectID)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not list role bindings", err, nil)
	}
	return c.JSON(http.StatusOK, RoleBindingsResponse{Bindings: bindings})
}

func (h *Handler) HandleCreateRoleBinding(c echo.Context) error {
	var req CreateRoleBindingRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	binding, err := h.c.BindRole(c.Request().Context(), req.SubjectType, req.SubjectID, req.Role, req.ScopeGroupUUID)
	if err != nil {
		if errors.Is(err, core.ErrInvalidRole) {
			return wrapError(http.StatusBadRequest, "invalid role", err, nil)
		}
		return wrapError(http.StatusInternalServerError, "could not create role binding", err, nil)
	}
	return c.JSON(http.StatusCreated, binding)
}

func (h *Handler) HandleDeleteRoleBinding(c echo.Context) error {
	var req GetRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	if err := h.c.UnbindRole(c.Request().Context(), req.ID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return wrapError(http.StatusNotFound, "role binding not found", err, nil)
		}
		return wrapError(http.StatusInternalServerError, "could not delete role binding", err, nil)
	}
	return c.NoContent(http.StatusOK)
}
