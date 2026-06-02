package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"github.com/cvhariharan/checkpost/internal/models"
	"github.com/labstack/echo/v4"
	"github.com/lib/pq"
)

func (h *Handler) HandleCreateOwner(c echo.Context) error {
	var req CreateOwnerRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	owner, err := h.c.CreateDeviceOwner(c.Request().Context(), models.CreateDeviceOwner{
		DisplayName: req.DisplayName,
		Email:       req.Email,
		ExternalID:  req.ExternalID,
		Department:  req.Department,
		Title:       req.Title,
		Phone:       req.Phone,
		Notes:       req.Notes,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return wrapError(http.StatusConflict, "owner email or external id already exists", err, nil)
		}
		return wrapError(http.StatusInternalServerError, "error creating owner", err, nil)
	}

	return c.JSON(http.StatusCreated, CreateResponse{ID: owner.UUID})
}

func (h *Handler) HandleOwnersPagination(c echo.Context) error {
	var req OwnersRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	if req.Page > 0 {
		req.Page -= 1
	}
	if req.Count == 0 {
		req.Count = CountPerPage
	}

	page, err := h.c.PaginateDeviceOwners(c.Request().Context(), models.DeviceOwnerListRequest{
		Page:  req.Page,
		Count: req.Count,
		Query: req.Query,
	})
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not get owners", err, nil)
	}

	return c.JSON(http.StatusOK, PaginateOwnersResponse{
		Owners:     page.Items,
		TotalCount: page.TotalCount,
		PageCount:  page.PageCount,
	})
}

func (h *Handler) HandleGetOwner(c echo.Context) error {
	var req GetRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	owner, err := h.c.GetDeviceOwner(c.Request().Context(), models.ResourceID{UUID: req.ID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return wrapError(http.StatusNotFound, fmt.Sprintf("owner %s not found", req.ID), err, nil)
		}
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error getting owner %s", req.ID), err, nil)
	}

	return c.JSON(http.StatusOK, owner)
}

func (h *Handler) HandleUpdateOwner(c echo.Context) error {
	var req UpdateOwnerRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	owner, err := h.c.UpdateDeviceOwner(c.Request().Context(), models.UpdateDeviceOwner{
		UUID:        req.ID,
		DisplayName: req.DisplayName,
		Email:       req.Email,
		ExternalID:  req.ExternalID,
		Department:  req.Department,
		Title:       req.Title,
		Phone:       req.Phone,
		Notes:       req.Notes,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return wrapError(http.StatusNotFound, fmt.Sprintf("owner %s not found", req.ID), err, nil)
		}
		if isUniqueViolation(err) {
			return wrapError(http.StatusConflict, "owner email or external id already exists", err, nil)
		}
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error updating owner %s", req.ID), err, nil)
	}

	return c.JSON(http.StatusOK, owner)
}

func (h *Handler) HandleDeleteOwner(c echo.Context) error {
	var req GetRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	if err := h.c.DeleteDeviceOwner(c.Request().Context(), models.ResourceID{UUID: req.ID}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return wrapError(http.StatusNotFound, fmt.Sprintf("owner %s not found", req.ID), err, nil)
		}
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error deleting owner %s", req.ID), err, nil)
	}

	return c.NoContent(http.StatusOK)
}

func (h *Handler) HandleOwnerMachines(c echo.Context) error {
	var req OwnerMachinesRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	if req.Page > 0 {
		req.Page -= 1
	}
	if req.Count == 0 {
		req.Count = CountPerPage
	}

	page, err := h.c.PaginateOwnerMachines(c.Request().Context(), models.OwnerMachinesRequest{
		OwnerUUID: req.ID,
		Page:      req.Page,
		Count:     req.Count,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return wrapError(http.StatusNotFound, fmt.Sprintf("owner %s not found", req.ID), err, nil)
		}
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error getting owner machines %s", req.ID), err, nil)
	}

	return c.JSON(http.StatusOK, OwnerMachinesResponse{
		Machines:   page.Items,
		TotalCount: page.TotalCount,
		PageCount:  page.PageCount,
	})
}

func (h *Handler) HandleMachineInventory(c echo.Context) error {
	var req GetRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	inventory, err := h.c.GetNodeInventory(c.Request().Context(), models.NodeIdentity{ID: req.ID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return wrapError(http.StatusNotFound, fmt.Sprintf("machine %s not found", req.ID), err, nil)
		}
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error getting machine inventory %s", req.ID), err, nil)
	}

	return c.JSON(http.StatusOK, MachineInventoryResponse{Inventory: inventory})
}

func (h *Handler) HandleUpdateMachineInventory(c echo.Context) error {
	var req UpdateMachineInventoryRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	ownerID := ""
	if req.OwnerID != nil {
		ownerID = *req.OwnerID
	}

	inventory, err := h.c.UpdateNodeInventory(c.Request().Context(), models.UpdateNodeInventory{
		NodeUUID:           req.ID,
		OwnerUUID:          ownerID,
		InternalTrackingID: req.InternalTrackingID,
		Notes:              req.Notes,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return wrapError(http.StatusNotFound, "machine or owner not found", err, nil)
		}
		if isUniqueViolation(err) {
			return wrapError(http.StatusConflict, "internal tracking id already exists", err, nil)
		}
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error updating machine inventory %s", req.ID), err, nil)
	}

	return c.JSON(http.StatusOK, MachineInventoryResponse{Inventory: inventory})
}

func (h *Handler) HandleDeleteMachineInventory(c echo.Context) error {
	var req GetRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	if err := h.c.DeleteNodeInventory(c.Request().Context(), models.NodeIdentity{ID: req.ID}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return wrapError(http.StatusNotFound, fmt.Sprintf("machine %s not found", req.ID), err, nil)
		}
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error deleting machine inventory %s", req.ID), err, nil)
	}

	return c.NoContent(http.StatusOK)
}

func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && string(pqErr.Code) == "23505"
}
