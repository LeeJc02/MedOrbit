package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ddi/internal/audit"
	"ddi/internal/config"
	handlers "ddi/internal/httpapi"
	"ddi/internal/middleware"
	"ddi/internal/orchestrator"
	"ddi/internal/runtimeclient"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg := config.Load()

	client, err := runtimeclient.New(cfg.GRPCAddr)
	if err != nil {
		log.Fatalf("grpc connect: %v", err)
	}
	defer client.Close()

	logger := audit.Logger(audit.NoopLogger{})
	if cfg.AuditEnabled {
		pool, err := pgxpool.New(context.Background(), cfg.PostgresDSN)
		if err != nil {
			log.Fatalf("pg connect: %v", err)
		}
		defer pool.Close()
		if err := audit.EnsureSchema(context.Background(), pool); err != nil {
			log.Fatalf("pg migrate audit schema: %v", err)
		}
		logger = audit.NewPostgresLogger(pool)
	}

	service := orchestrator.New(client, logger)
	h := handlers.New(service)

	router := gin.Default()
	router.StaticFile("/", "./web/index.html")
	router.StaticFile("/styles.css", "./web/styles.css")
	router.StaticFile("/app.js", "./web/app.js")
	router.StaticFile("/docs", "./web/openapi.html")
	router.StaticFile("/openapi.yaml", "./openapi/openapi.yaml")
	router.Static("/assets", "./assets")

	api := router.Group("/v1")
	api.Use(middleware.JWTMiddleware(cfg.JWTSecret))
	api.Use(middleware.RBACMiddleware("doctor"))
	api.POST("/session/run", h.RunSession)
	api.POST("/session/replay", h.Replay)

	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: router}

	go func() {
		log.Printf("gateway listening on %s", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}
