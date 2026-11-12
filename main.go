package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/attachaiB/backend-challenge/internal/config"
	"github.com/attachaiB/backend-challenge/internal/controller"
	"github.com/attachaiB/backend-challenge/internal/database"
	"github.com/attachaiB/backend-challenge/internal/middleware"
	"github.com/attachaiB/backend-challenge/internal/repository"
	"github.com/attachaiB/backend-challenge/internal/routes"
	"github.com/attachaiB/backend-challenge/internal/service"
	"github.com/attachaiB/backend-challenge/internal/worker"
)

func main() {
	// ctx is cancelled when the process receives Ctrl+C (SIGINT) or SIGTERM from Docker.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	client, err := database.Connect(ctx, cfg.MongoURI)
	if err != nil {
		log.Fatal(err)
	}

	db := client.Database(cfg.MongoDatabase)
	if err := database.EnsureIndexes(ctx, db); err != nil {
		log.Fatal(err)
	}

	// Dependency injection: each layer only receives the interfaces it needs.
	userRepository := repository.NewUserRepository(db)
	authService := service.NewAuthService(userRepository, cfg.JWTSecret, cfg.JWTDuration)
	userService := service.NewUserService(userRepository)

	router := routes.SetupRouter(routes.Handlers{
		Auth:           controller.NewAuthController(authService),
		User:           controller.NewUserController(userService),
		AuthMiddleware: middleware.Auth(authService),
	})

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		worker.RunUserCounter(ctx, userService, cfg.UserCountInterval)
	}()

	go func() {
		log.Printf("listening on :%s", cfg.Port)

		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("http server: %v", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Println("shutting down")

	// A new context is needed here because ctx is already cancelled.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	// Shutdown stops accepting new requests and waits for running ones to finish.
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("http shutdown: %v", err)
	}

	wg.Wait()

	if err := client.Disconnect(shutdownCtx); err != nil {
		log.Printf("mongo disconnect: %v", err)
	}

	log.Println("stopped")
}
