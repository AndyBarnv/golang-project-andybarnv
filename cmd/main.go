package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	pb "github.com/AndyBarnv/golang-project-andybarnv/gen/moviesv1"
	"github.com/AndyBarnv/golang-project-andybarnv/internal/adapter/httpstand"
	"github.com/AndyBarnv/golang-project-andybarnv/internal/adapter/postgres"
	"github.com/AndyBarnv/golang-project-andybarnv/internal/config"
	"github.com/AndyBarnv/golang-project-andybarnv/internal/handler/grpcserver"
	"github.com/AndyBarnv/golang-project-andybarnv/internal/handler/httpserver"
	"github.com/AndyBarnv/golang-project-andybarnv/internal/usecase"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctxSig, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	poolCfg, err := pgxpool.ParseConfig(cfg.DataBaseURL)
	if err != nil {
		return fmt.Errorf("failed to parse database config: %w", err)
	}
	poolCfg.ConnConfig.ConnectTimeout = config.ConnectTimeout

	pool, err := pgxpool.NewWithConfig(ctxSig, poolCfg)
	if err != nil {
		return fmt.Errorf("failed to make connection to database: %w", err)
	}
	defer pool.Close()

	pingCtx, cancelPing := context.WithTimeout(ctxSig, config.ConnectTimeout)
	err = pool.Ping(pingCtx)
	cancelPing()
	if err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	if err := postgres.Migrate(ctxSig, pool); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}

	movieClient := httpstand.NewClient(cfg.MovieBaseURL, config.ClientTimeout)
	movieRepository := postgres.NewRepository(pool)
	svc, err := usecase.NewService(movieClient, movieRepository, config.WorkersNum)
	if err != nil {
		return err
	}

	syncCtx, cancelSync := context.WithTimeout(ctxSig, config.SyncTimeout)
	err = svc.SyncDatabase(syncCtx)
	cancelSync()
	if err != nil {
		return fmt.Errorf("sync database: %w", err)
	}

	mux := httpserver.NewHandler(svc)
	httpServ := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: config.ServerReadHeaderTimeout,
		ReadTimeout:       config.ServerReadTimeout,
		WriteTimeout:      config.ServerWriteTimeout,
		IdleTimeout:       config.ServerIdleTimeout,
	}

	grpcLogger := log.New(os.Stderr, "grpc server ", log.Ltime|log.Lmicroseconds)
	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", cfg.GRPCAddr, err)
	}

	grpcServ := grpc.NewServer(
		grpc.UnaryInterceptor(grpcserver.Logging(grpcLogger)),
	)
	pb.RegisterMovieServiceServer(grpcServ, grpcserver.NewMovieServer(svc))
	reflection.Register(grpcServ)

	errCh := make(chan error, 2)

	go func() {
		log.Printf("http server listening on %s", httpServ.Addr)
		if err := httpServ.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("http server error: %w", err)
		}
	}()

	go func() {
		log.Printf("grpc server listening on %s", cfg.GRPCAddr)
		if err := grpcServ.Serve(lis); err != nil {
			errCh <- fmt.Errorf("grpc server error: %w", err)
		}
	}()

	select {
	case err := <-errCh:
		log.Printf("server failure: %v", err)
	case <-ctxSig.Done():
		log.Printf("shutdown signal received")
	}
	stop()

	ctxShutdown, cancel := context.WithTimeout(context.Background(), config.ShutdownTimeout)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		log.Printf("http server stopping")
		if err := httpServ.Shutdown(ctxShutdown); err != nil {
			httpServ.Close()
			log.Printf("http server graceful shutdown failed: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		log.Printf("grpc server stopping")
		timer := time.AfterFunc(config.ShutdownTimeout, grpcServ.Stop)
		defer timer.Stop()
		grpcServ.GracefulStop()
	}()

	wg.Wait()
	log.Printf("all servers stopped")
	return nil
}
