package main

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/ranit-biswas/taskflow/internal/config"
	"github.com/ranit-biswas/taskflow/internal/database"
	"github.com/ranit-biswas/taskflow/internal/handler"
	"github.com/ranit-biswas/taskflow/internal/middleware"
	"github.com/ranit-biswas/taskflow/internal/repository"
	"go.uber.org/zap"
)

func main() {
	_ = godotenv.Load()

	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg := config.Load()

	db, err := connectWithRetry(cfg.DB, logger)
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	userRepo := repository.NewUserRepo(db)
	projectRepo := repository.NewProjectRepo(db)
	taskRepo := repository.NewTaskRepo(db)

	authHandler := handler.NewAuthHandler(userRepo, cfg.JWTSecret, logger)
	projectHandler := handler.NewProjectHandler(projectRepo, taskRepo, logger)
	taskHandler := handler.NewTaskHandler(taskRepo, projectRepo, userRepo, logger)

	e := echo.New()
	e.HideBanner = true

	e.Use(echomw.Recover())
	e.Use(echomw.RequestID())
	e.Use(echomw.CORSWithConfig(echomw.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowHeaders: []string{echo.HeaderContentType, echo.HeaderAuthorization},
	}))

	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	auth := e.Group("/auth")
	auth.POST("/register", authHandler.Register)
	auth.POST("/login", authHandler.Login)

	api := e.Group("")
	api.Use(middleware.JWTAuth(cfg.JWTSecret))

	api.GET("/projects", projectHandler.List)
	api.POST("/projects", projectHandler.Create)
	api.GET("/projects/:id", projectHandler.Get)
	api.PATCH("/projects/:id", projectHandler.Update)
	api.DELETE("/projects/:id", projectHandler.Delete)
	api.GET("/projects/:id/stats", projectHandler.Stats)

	api.GET("/projects/:id/tasks", taskHandler.List)
	api.POST("/projects/:id/tasks", taskHandler.Create)
	api.PATCH("/tasks/:id", taskHandler.Update)
	api.DELETE("/tasks/:id", taskHandler.Delete)

	go func() {
		addr := ":" + cfg.Port
		logger.Info("server starting", zap.String("addr", addr))
		if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		logger.Error("server forced to shutdown", zap.Error(err))
	}
	logger.Info("server stopped")
}

func connectWithRetry(cfg config.DBConfig, logger *zap.Logger) (*sql.DB, error) {
	var db *sql.DB
	var err error
	for i := range 30 {
		db, err = database.Connect(cfg)
		if err == nil {
			return db, nil
		}
		logger.Info("waiting for database", zap.Int("attempt", i+1), zap.Error(err))
		time.Sleep(time.Second)
	}
	return nil, err
}
