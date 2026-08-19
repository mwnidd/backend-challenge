package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	mongoadapter "github.com/7-solutions/backend-challenge/internal/adapters/mongo"
	"github.com/7-solutions/backend-challenge/internal/application"
	"github.com/7-solutions/backend-challenge/internal/auth"
	"github.com/7-solutions/backend-challenge/internal/background"
	"github.com/7-solutions/backend-challenge/internal/httpapi"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	cfg := loadConfig()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		log.Fatalf("connect mongo: %v", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := mongoClient.Disconnect(shutdownCtx); err != nil {
			log.Printf("disconnect mongo: %v", err)
		}
	}()

	if err := mongoClient.Ping(ctx, nil); err != nil {
		log.Fatalf("ping mongo: %v", err)
	}

	repo := mongoadapter.NewUserRepository(mongoClient.Database(cfg.MongoDatabase), "users")
	if err := repo.EnsureIndexes(ctx); err != nil {
		log.Fatalf("ensure indexes: %v", err)
	}

	jwtManager := auth.NewJWTManager(cfg.JWTSecret, cfg.JWTTTL)
	userService := application.NewUserService(repo, jwtManager)
	background.StartUserCountLogger(ctx, userService, 10*time.Second)

	handler := httpapi.NewHandler(userService)
	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler.Routes(func(next http.Handler) http.Handler { return httpapi.Authenticate(jwtManager, next) }),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("api listening on :%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown: %v", err)
	}
}

type config struct {
	Port          string
	MongoURI      string
	MongoDatabase string
	JWTSecret     string
	JWTTTL        time.Duration
}

func loadConfig() config {
	ttlHours := getenvInt("JWT_TTL_HOURS", 24)
	return config{
		Port:          getenv("PORT", "8080"),
		MongoURI:      getenv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDatabase: getenv("MONGO_DATABASE", "backend_challenge"),
		JWTSecret:     getenvRequired("JWT_SECRET"),
		JWTTTL:        time.Duration(ttlHours) * time.Hour,
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getenvRequired(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("%s is required", key)
	}
	return value
}
