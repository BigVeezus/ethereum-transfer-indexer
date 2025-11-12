package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"pagrin/internal/cache"
	"pagrin/internal/config"
	"pagrin/internal/ethereum"
	"pagrin/internal/handler"
	"pagrin/internal/repository"
	"pagrin/internal/service"
	"pagrin/internal/stream"
	"pagrin/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(
		cfg.Logging.Level,
		cfg.Logging.ToFile,
		cfg.Logging.FilePath,
		cfg.Logging.Format,
	)

	// Initialize Ethereum client with provider pool (if configured) or single provider
	var ethereumClient *ethereum.Client
	if cfg.Ethereum.RPCConfig != "" {
		// Load providers from YAML config (preferred - supports failover)
		providers, err := config.LoadProvidersFromYAML(cfg.Ethereum.RPCConfig, cfg.Ethereum.RPCURL)
		if err != nil {
			log.Error("Failed to load providers from config: %v", err)
			os.Exit(1)
		}

		pool := ethereum.NewProviderPool(providers)
		ethereumClient = ethereum.NewClientFromPool(pool)
		log.Info("Initialized provider pool with %d providers", len(providers))
	} else {
		// Fallback to single provider (legacy mode)
		ethereumClient, err = ethereum.NewClient(cfg.Ethereum.RPCURL)
		if err != nil {
			log.Error("Failed to create Ethereum client: %v", err)
			os.Exit(1)
		}
		log.Info("Using single RPC provider (legacy mode)")
	}
	defer ethereumClient.Close()

	fetcher := ethereum.NewFetcher(ethereumClient)

	// Initialize Redis cache (optional, gracefully degrades if unavailable)
	var redisCache cache.Cache
	if cfg.Redis.Enabled {
		redisCache, err = cache.NewRedisCache(cfg.Redis.URI, true, log)
		if err != nil {
			log.Warn("Redis cache unavailable, continuing with MongoDB-only mode: %v", err)
			redisCache = nil
		} else {
			defer func() {
				if err := redisCache.Close(); err != nil {
					log.Error("Failed to close Redis cache: %v", err)
				}
			}()
		}
	}

	repo, err := repository.NewMongoRepository(cfg.MongoDB.URI, cfg.MongoDB.Database, redisCache)
	if err != nil {
		log.Error("Failed to create repository: %v", err)
		os.Exit(1)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := repo.Close(ctx); err != nil {
			log.Error("Failed to close repository: %v", err)
		}
	}()

	transferService := service.NewTransferService(repo, log)

	// Initialize streaming if enabled
	// Keep concrete type for handler, use interface for service
	var streamInstance *stream.Stream
	var streamPublisher service.StreamPublisher
	if cfg.Streaming.Enabled {
		streamInstance = stream.NewStream(cfg.Streaming.BufferSize, log)
		streamPublisher = streamInstance // Assign to interface for service
		log.Info("Streaming enabled: type=%s, route=%s", cfg.Streaming.Type, cfg.Streaming.Route)
	}
	// When disabled, streamPublisher is nil interface, so nil check works correctly

	ingestionService := service.NewIngestionService(
		ethereumClient,
		fetcher,
		repo,
		log,
		cfg.Ingestion.PollInterval,
		cfg.Ingestion.StartBlock,
		cfg.Ingestion.BlockBatchSize,
		cfg.Ingestion.ResetStartBlock,
		cfg.Ingestion.AdaptiveBatch,
		cfg.Ingestion.BatchMinSize,
		cfg.Ingestion.BatchMaxSize,
		cfg.Ingestion.BatchSuccessStreak,
		cfg.Ingestion.BatchFailureBackoff,
		streamPublisher,
	)

	transferHandler := handler.NewTransferHandler(transferService)

	// Create stream handler if streaming is enabled
	var streamHandler *handler.StreamHandler
	if cfg.Streaming.Enabled && streamInstance != nil {
		streamHandler = handler.NewStreamHandler(streamInstance)
	}

	router := setupRouter(transferHandler, streamHandler, cfg)

	server := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: router,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := ingestionService.Start(ctx); err != nil {
			log.Error("Ingestion service error: %v", err)
		}
	}()

	go func() {
		log.Info("Starting HTTP server on port %s", cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("HTTP server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("Server forced to shutdown: %v", err)
	}

	log.Info("Server exited")
}

func setupRouter(transferHandler *handler.TransferHandler, streamHandler *handler.StreamHandler, cfg *config.Config) *gin.Engine {
	router := gin.Default()

	api := router.Group("/api/v1")
	{
		api.GET("/transfers", transferHandler.GetTransfers)
		api.GET("/aggregates", transferHandler.GetAggregates)
	}

	// Streaming endpoints (if enabled)
	if cfg.Streaming.Enabled && streamHandler != nil {
		if cfg.Streaming.Type == "ws" {
			router.GET(cfg.Streaming.Route, streamHandler.HandleWebSocket)
		} else if cfg.Streaming.Type == "sse" {
			router.GET(cfg.Streaming.Route, streamHandler.HandleSSE)
		}
	}

	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	return router
}
