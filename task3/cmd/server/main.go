package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/AndyBarnv/golang-project-andybarnv/task3/internal/client"
	"github.com/AndyBarnv/golang-project-andybarnv/task3/internal/config"
	"github.com/AndyBarnv/golang-project-andybarnv/task3/internal/server"
	"github.com/AndyBarnv/golang-project-andybarnv/task3/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	ctxSig, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	movieClient := client.NewClient(cfg.MovieBaseURL, config.ClientTimeout)
	svc := service.NewService(movieClient, config.WorkersNum)
	mux := server.NewHandler(svc)

	httpServ := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: config.ServerReadHeaderTimeout,
		ReadTimeout:       config.ServerReadTimeout,
		WriteTimeout:      config.ServerWriteTimeout,
		IdleTimeout:       config.ServerIdleTimeout,
	}

	errCh := make(chan error, 1)

	go func() {
		log.Printf("HTTP server listening on %s", httpServ.Addr)
		if err := httpServ.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		log.Fatalf("HTTP server failure: %v", err)
	case <-ctxSig.Done():
		log.Printf("server forced to shutdown")
		stop()

		ctxShutdown, cancel := context.WithTimeout(context.Background(), config.ShutdownTimeout)
		defer cancel()
		if err := httpServ.Shutdown(ctxShutdown); err != nil {
			httpServ.Close()
			log.Fatalf("HTTP server graceful shutdown failed: %v", err)
		}
		log.Printf("server stopped")
	}
}
