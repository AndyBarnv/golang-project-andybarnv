package grpcserver

import (
	"context"
	"errors"
	"log"
	"time"

	pb "github.com/AndyBarnv/golang-project-andybarnv/task3/gen/moviesv1"
	"github.com/AndyBarnv/golang-project-andybarnv/task3/internal/client"
	"github.com/AndyBarnv/golang-project-andybarnv/task3/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Service interface {
	LoadMovies(ctx context.Context, from, to int) ([]service.MovieResp, error)
	LastMovie(ctx context.Context) (client.Movie, error)
}

type MovieServer struct {
	pb.UnimplementedMovieServiceServer
	svc Service
}

func NewMovieServer(svc Service) *MovieServer {
	return &MovieServer{svc: svc}
}

func Logging(logger *log.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		if err != nil {
			logger.Printf(
				"method=%s code=%s duration=%s error=%q",
				info.FullMethod,
				status.Code(err),
				time.Since(start),
				status.Convert(err).Message(),
			)
		} else {
			logger.Printf("method=%s code=%s duration=%s", info.FullMethod, status.Code(err), time.Since(start))
		}
		return resp, err
	}
}

func toProtoMovie(m client.Movie) *pb.Movie {
	return &pb.Movie{
		Id:       int64(m.ID),
		Title:    m.Title,
		Year:     int32(m.Year),
		Director: m.Director,
	}
}

func toProtoMovieItem(m service.MovieResp) *pb.MovieItem {
	if m.Error != nil {
		return &pb.MovieItem{
			Result: &pb.MovieItem_Error{
				Error: &pb.ItemError{Id: int64(m.ID), Error: m.Error.Error()},
			},
		}
	}
	return &pb.MovieItem{
		Result: &pb.MovieItem_Movie{
			Movie: toProtoMovie(m.MovieInfo),
		},
	}
}

func (s *MovieServer) LoadMovies(ctx context.Context, req *pb.LoadMoviesRequest) (*pb.LoadMoviesResponse, error) {
	items, err := s.svc.LoadMovies(ctx, int(req.GetFrom()), int(req.GetTo()))
	if err != nil {
		if ctx.Err() != nil {
			return nil, status.FromContextError(ctx.Err()).Err()
		}
		switch {
		case errors.Is(err, service.ErrBadRange):
			return nil, status.Error(codes.InvalidArgument, err.Error())
		default:
			return nil, status.Error(codes.Unavailable, err.Error())
		}
	}

	protoMovieItems := make([]*pb.MovieItem, 0, len(items))
	for i := 0; i < len(items); i++ {
		protoMovieItems = append(protoMovieItems, toProtoMovieItem(items[i]))
	}

	return &pb.LoadMoviesResponse{Items: protoMovieItems}, nil
}

func (s *MovieServer) GetLastMovie(ctx context.Context, req *pb.GetLastMovieRequest) (*pb.Movie, error) {
	movie, err := s.svc.LastMovie(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return nil, status.FromContextError(ctx.Err()).Err()
		}
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			return nil, status.Error(codes.DeadlineExceeded, err.Error())
		case errors.Is(err, client.ErrNotFound):
			return nil, status.Error(codes.NotFound, err.Error())
		default:
			return nil, status.Error(codes.Unavailable, err.Error())
		}
	}
	return toProtoMovie(movie), nil
}
