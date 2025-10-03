package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"recon-engine/internal/service"
	"recon-engine/pkg/logger"
	"recon-engine/pkg/response"
)

type ReconciliationHandler struct {
	service service.ReconciliationService
}

func NewReconciliationHandler(service service.ReconciliationService) *ReconciliationHandler {
	return &ReconciliationHandler{service: service}
}

type ReconcileRequest struct {
	SystemFilePath string   `json:"system_file_path"`
	BankFilePaths  []string `json:"bank_file_paths" binding:"required,min=1"`
	StartDate      string   `json:"start_date" binding:"required"`
	EndDate        string   `json:"end_date" binding:"required"`
}

// Reconcile godoc
// @Summary Perform reconciliation
// @Description Reconcile system transactions with bank statements
// @Tags reconciliation
// @Accept json
// @Produce json
// @Param request body ReconcileRequest true "Reconciliation request"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /api/v1/reconcile [post]
func (h *ReconciliationHandler) Reconcile(c *gin.Context) {
	var req ReconcileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.GetLogger().WithError(err).Error("Invalid request")
		response.ValidationError(c, err.Error())
		return
	}

	// Parse dates
	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		response.BadRequest(c, "Invalid start_date format", "Use YYYY-MM-DD format")
		return
	}

	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		response.BadRequest(c, "Invalid end_date format", "Use YYYY-MM-DD format")
		return
	}

	// Set end date to end of day
	endDate = endDate.Add(23*time.Hour + 59*time.Minute + 59*time.Second)

	logger.GetLogger().WithFields(map[string]interface{}{
		"system_file":     req.SystemFilePath,
		"bank_files":      req.BankFilePaths,
		"start_date":      startDate,
		"end_date":        endDate,
	}).Info("Starting reconciliation")

	summary, err := h.service.Reconcile(req.SystemFilePath, req.BankFilePaths, startDate, endDate)
	if err != nil {
		logger.GetLogger().WithError(err).Error("Reconciliation failed")
		response.InternalError(c, "Reconciliation failed", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Reconciliation completed successfully", summary)
}

// GetJobStatus godoc
// @Summary Get reconciliation job status
// @Description Get the status of a reconciliation job by ID
// @Tags reconciliation
// @Produce json
// @Param job_id path string true "Job ID"
// @Success 200 {object} response.Response
// @Failure 404 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /api/v1/reconcile/jobs/{job_id} [get]
func (h *ReconciliationHandler) GetJobStatus(c *gin.Context) {
	jobID := c.Param("job_id")

	job, err := h.service.GetJobStatus(jobID)
	if err != nil {
		logger.GetLogger().WithError(err).WithField("job_id", jobID).Error("Job not found")
		response.NotFound(c, "Job not found")
		return
	}

	response.Success(c, http.StatusOK, "Job status retrieved successfully", job)
}

// GetJobSummary godoc
// @Summary Get reconciliation job summary
// @Description Get the detailed summary of a reconciliation job by ID
// @Tags reconciliation
// @Produce json
// @Param job_id path string true "Job ID"
// @Success 200 {object} response.Response
// @Failure 404 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /api/v1/reconcile/jobs/{job_id}/summary [get]
func (h *ReconciliationHandler) GetJobSummary(c *gin.Context) {
	jobID := c.Param("job_id")

	summary, err := h.service.GetJobSummary(jobID)
	if err != nil {
		logger.GetLogger().WithError(err).WithField("job_id", jobID).Error("Failed to get job summary")
		response.NotFound(c, "Job not found")
		return
	}

	response.Success(c, http.StatusOK, "Job summary retrieved successfully", summary)
}
