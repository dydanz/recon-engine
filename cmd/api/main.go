package main

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "recon-engine/docs"
	"recon-engine/internal/config"
	"recon-engine/internal/handler"
	"recon-engine/internal/middleware"
	"recon-engine/internal/repository"
	"recon-engine/internal/service"
	"recon-engine/pkg/logger"
)

// @title Transaction Reconciliation API
// @version 1.0
// @description API for reconciling transactions between system and bank statements
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.email support@recon-engine.com

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:8080
// @BasePath /
// @schemes http https

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	logger.Init(cfg.App.LogLevel)
	logger.GetLogger().Info("Starting Transaction Reconciliation Service")

	// Connect to database
	db, err := connectDB(cfg.Database)
	if err != nil {
		logger.GetLogger().WithError(err).Fatal("Failed to connect to database")
	}
	defer db.Close()

	logger.GetLogger().Info("Database connection established")

	// Initialize repositories
	txRepo := repository.NewTransactionRepository(db)
	reconRepo := repository.NewReconciliationRepository(db)

	// Initialize services
	txService := service.NewTransactionService(txRepo)
	reconService := service.NewReconciliationService(txRepo, reconRepo, cfg.App.BatchSize)

	// Initialize handlers
	txHandler := handler.NewTransactionHandler(txService)
	reconHandler := handler.NewReconciliationHandler(reconService)

	// Setup router
	router := setupRouter(txHandler, reconHandler)

	// Start server
	addr := fmt.Sprintf(":%s", cfg.Server.Port)
	logger.GetLogger().WithField("address", addr).Info("Server starting")

	if err := router.Run(addr); err != nil {
		logger.GetLogger().WithError(err).Fatal("Failed to start server")
	}
}

func connectDB(cfg config.DatabaseConfig) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.ConnectionString())
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	// Set connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	return db, nil
}

func setupRouter(txHandler *handler.TransactionHandler, reconHandler *handler.ReconciliationHandler) *gin.Engine {
	router := gin.New()

	// Global middleware
	router.Use(middleware.Recovery())
	router.Use(middleware.Logger())
	router.Use(middleware.ErrorHandler())
	router.Use(gin.Recovery())

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy"})
	})

	// Swagger documentation
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Transaction routes
		transactions := v1.Group("/transactions")
		{
			transactions.POST("", txHandler.CreateTransaction)
			transactions.POST("/bulk", txHandler.BulkCreateTransactions)
			transactions.GET("/:trx_id", txHandler.GetTransaction)
			transactions.GET("", txHandler.GetTransactionsByDateRange)
		}

		// Reconciliation routes
		reconciliation := v1.Group("/reconcile")
		{
			reconciliation.POST("", reconHandler.Reconcile)
			reconciliation.GET("/jobs/:job_id", reconHandler.GetJobStatus)
			reconciliation.GET("/jobs/:job_id/summary", reconHandler.GetJobSummary)
		}
	}

	return router
}
