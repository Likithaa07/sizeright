package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Likithaa07/sizeright/internal/api"
	"github.com/Likithaa07/sizeright/internal/sizing"
	"github.com/Likithaa07/sizeright/internal/store"
)

// getenv reads an environment variable with a default.
func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	rules, err := sizing.LoadRules(getenv("RULES_PATH", "rules/v1.yaml"))
	if err != nil {
		return err
	}
	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 32 {
		return errors.New("JWT_SECRET must be set and at least 32 characters")
	}
	ctx := context.Background()
	st, err := store.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	defer st.Close()

	srv := &http.Server{
		Addr:         ":" + getenv("PORT", "8080"),
		Handler:      (&api.Server{Rules: rules, Store: st, JWTSecret: []byte(secret), Log: log}).Routes(),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown: on Ctrl+C or SIGTERM, finish in-flight requests first.
	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
		<-stop
		log.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Info("listening", "addr", srv.Addr, "rules_version", rules.Version)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
