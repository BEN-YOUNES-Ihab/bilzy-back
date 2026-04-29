package main

import (
	"context"
	"errors"
	"log/slog"
	stdhttp "net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bilzy/bilzy-back/internal/config"
	"github.com/bilzy/bilzy-back/internal/db"
	"github.com/bilzy/bilzy-back/internal/db/store"
	"github.com/bilzy/bilzy-back/internal/firebase"
	apphttp "github.com/bilzy/bilzy-back/internal/http"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	})))

	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.NewPool(rootCtx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	st := store.New(pool)

	fb, err := firebase.New(rootCtx, cfg.FirebaseProjectID, cfg.GoogleCredsPath)
	if err != nil {
		return err
	}

	authMW := apphttp.NewAuthMiddleware(fb, st)

	router := apphttp.NewRouter(&apphttp.Deps{
		AuthMiddleware: authMW,
		CORSOrigins:    cfg.CORSOrigins,
		Profile:        apphttp.NewProfileHandler(st),
		Shops:          apphttp.NewShopHandler(st),
		Categories:     apphttp.NewCategoryHandler(st),
		Closings:       apphttp.NewClosingHandler(st),
	})

	srv := &stdhttp.Server{
		Addr:              cfg.Addr(),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		slog.Info("listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, stdhttp.ErrServerClosed) {
			slog.Error("listen", "err", err)
			stop()
		}
	}()

	<-rootCtx.Done()
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
