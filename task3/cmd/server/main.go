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

	pb "github.com/AndyBarnv/golang-project-andybarnv/task3/gen/moviesv1"
	"github.com/AndyBarnv/golang-project-andybarnv/task3/internal/client"
	"github.com/AndyBarnv/golang-project-andybarnv/task3/internal/config"
	"github.com/AndyBarnv/golang-project-andybarnv/task3/internal/grpcserver"
	"github.com/AndyBarnv/golang-project-andybarnv/task3/internal/httpserver"
	"github.com/AndyBarnv/golang-project-andybarnv/task3/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
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
		grpcLogger.Fatalf("failed to listen on %s: %v", cfg.GRPCAddr, err)
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
}
