package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"recon-engine/internal/domain"
	"recon-engine/internal/service"
	"recon-engine/pkg/logger"
	"recon-engine/pkg/response"
)

type TransactionHandler struct {
	service service.TransactionService
}

func NewTransactionHandler(service service.TransactionService) *TransactionHandler {
	return &TransactionHandler{service: service}
}

type CreateTransactionRequest struct {
	TrxID           string  `json:"trx_id" binding:"required"`
	Amount          float64 `json:"amount" binding:"required,gt=0"`
	Type            string  `json:"type" binding:"required,oneof=DEBIT CREDIT"`
	TransactionTime string  `json:"transaction_time" binding:"required"`
}

type BulkCreateTransactionRequest struct {
	Transactions []CreateTransactionRequest `json:"transactions" binding:"required,min=1"`
}

type GetTransactionsByDateRangeRequest struct {
	StartDate string `form:"start_date" binding:"required"`
	EndDate   string `form:"end_date" binding:"required"`
}

// CreateTransaction godoc
// @Summary Create a new transaction
// @Description Create a new transaction in the system
// @Tags transactions
// @Accept json
// @Produce json
// @Param transaction body CreateTransactionRequest true "Transaction data"
// @Success 201 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /api/v1/transactions [post]
func (h *TransactionHandler) CreateTransaction(c *gin.Context) {
	var req CreateTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.GetLogger().WithError(err).Error("Invalid request")
		response.ValidationError(c, err.Error())
		return
	}

	transactionTime, err := time.Parse(time.RFC3339, req.TransactionTime)
	if err != nil {
		response.BadRequest(c, "Invalid transaction time format", "Use RFC3339 format")
		return
	}

	tx := &domain.Transaction{
		TrxID:           req.TrxID,
		Amount:          decimal.NewFromFloat(req.Amount),
		Type:            domain.TransactionType(req.Type),
		TransactionTime: transactionTime,
	}

	if err := h.service.Create(tx); err != nil {
		logger.GetLogger().WithError(err).Error("Failed to create transaction")
		response.InternalError(c, "Failed to create transaction", err.Error())
		return
	}

	response.Success(c, http.StatusCreated, "Transaction created successfully", tx)
}

// BulkCreateTransactions godoc
// @Summary Bulk create transactions
// @Description Create multiple transactions at once
// @Tags transactions
// @Accept json
// @Produce json
// @Param transactions body BulkCreateTransactionRequest true "Transactions data"
// @Success 201 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /api/v1/transactions/bulk [post]
func (h *TransactionHandler) BulkCreateTransactions(c *gin.Context) {
	var req BulkCreateTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	transactions := make([]domain.Transaction, 0, len(req.Transactions))
	for _, txReq := range req.Transactions {
		transactionTime, err := time.Parse(time.RFC3339, txReq.TransactionTime)
		if err != nil {
			logger.GetLogger().WithError(err).WithField("trx_id", txReq.TrxID).Warn("Invalid transaction time")
			continue
		}

		transactions = append(transactions, domain.Transaction{
			TrxID:           txReq.TrxID,
			Amount:          decimal.NewFromFloat(txReq.Amount),
			Type:            domain.TransactionType(txReq.Type),
			TransactionTime: transactionTime,
		})
	}

	if err := h.service.BulkCreate(transactions); err != nil {
		logger.GetLogger().WithError(err).Error("Failed to bulk create transactions")
		response.InternalError(c, "Failed to bulk create transactions", err.Error())
		return
	}

	response.Success(c, http.StatusCreated, "Transactions created successfully", map[string]int{"count": len(transactions)})
}

// GetTransaction godoc
// @Summary Get transaction by ID
// @Description Get a single transaction by its ID
// @Tags transactions
// @Produce json
// @Param trx_id path string true "Transaction ID"
// @Success 200 {object} response.Response
// @Failure 404 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /api/v1/transactions/{trx_id} [get]
func (h *TransactionHandler) GetTransaction(c *gin.Context) {
	trxID := c.Param("trx_id")

	tx, err := h.service.GetByTrxID(trxID)
	if err != nil {
		logger.GetLogger().WithError(err).WithField("trx_id", trxID).Error("Transaction not found")
		response.NotFound(c, "Transaction not found")
		return
	}

	response.Success(c, http.StatusOK, "Transaction retrieved successfully", tx)
}

// GetTransactionsByDateRange godoc
// @Summary Get transactions by date range
// @Description Get all transactions within a specified date range
// @Tags transactions
// @Produce json
// @Param start_date query string true "Start date (RFC3339 format)"
// @Param end_date query string true "End date (RFC3339 format)"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /api/v1/transactions [get]
func (h *TransactionHandler) GetTransactionsByDateRange(c *gin.Context) {
	var req GetTransactionsByDateRangeRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	startDate, err := time.Parse(time.RFC3339, req.StartDate)
	if err != nil {
		response.BadRequest(c, "Invalid start_date format", "Use RFC3339 format")
		return
	}

	endDate, err := time.Parse(time.RFC3339, req.EndDate)
	if err != nil {
		response.BadRequest(c, "Invalid end_date format", "Use RFC3339 format")
		return
	}

	transactions, err := h.service.GetByDateRange(startDate, endDate)
	if err != nil {
		logger.GetLogger().WithError(err).Error("Failed to get transactions")
		response.InternalError(c, "Failed to get transactions", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Transactions retrieved successfully", transactions)
}
