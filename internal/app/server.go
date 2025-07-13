package app

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"

	pb "github.com/example/auth/api"
	"github.com/example/auth/internal/handler"
)

// Server structure

type Server struct {
	HTTP *http.Server
	GRPC *grpc.Server
}

func New(db *sqlx.DB, rdb *redis.Client) *Server {
	h := handler.New(db, rdb)
	router := mux.NewRouter()
	h.Register(router)

	httpSrv := &http.Server{Handler: router}
	grpcSrv := grpc.NewServer()

	pb.RegisterAuthServer(grpcSrv, &grpcService{Handler: h})

	return &Server{HTTP: httpSrv, GRPC: grpcSrv}
}

func (s *Server) RunHTTP(addr string) error {
	s.HTTP.Addr = addr
	return s.HTTP.ListenAndServe()
}

func (s *Server) RunGRPC(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return s.GRPC.Serve(lis)
}

func (s *Server) Shutdown(ctx context.Context) {
	s.HTTP.Shutdown(ctx)
	s.GRPC.GracefulStop()
}

type grpcService struct {
	*handler.Handler
	pb.UnimplementedAuthServer
}

func (g *grpcService) SignUp(ctx context.Context, req *pb.SignUpRequest) (*pb.SignUpResponse, error) {
	w := &responseWriter{}
	body := newNopCloser(req)
	r := &http.Request{Body: &body, Header: http.Header{"Content-Type": {"application/json"}}}
	g.Handler.SignUp(w, r)
	if w.err != nil {
		return nil, w.err
	}
	return &pb.SignUpResponse{Id: w.id}, nil
}

func (g *grpcService) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	w := &respTokenWriter{}
	body := newNopCloser(req)
	r := &http.Request{Body: &body, Header: http.Header{"Content-Type": {"application/json"}}}
	g.Handler.Login(w, r)
	if w.err != nil {
		return nil, w.err
	}
	return &pb.LoginResponse{Access: w.access, Refresh: w.refresh}, nil
}

func (g *grpcService) Refresh(ctx context.Context, req *pb.RefreshRequest) (*pb.LoginResponse, error) {
	w := &respTokenWriter{}
	body := newNopCloser(req)
	r := &http.Request{Body: &body, Header: http.Header{"Content-Type": {"application/json"}}}
	g.Handler.Refresh(w, r)
	if w.err != nil {
		return nil, w.err
	}
	return &pb.LoginResponse{Access: w.access, Refresh: w.refresh}, nil
}

func (g *grpcService) Logout(ctx context.Context, req *pb.LogoutRequest) (*pb.LogoutResponse, error) {
	w := &respStatusWriter{}
	body := newNopCloser(req)
	r := &http.Request{Body: &body, Header: http.Header{"Content-Type": {"application/json"}}}
	g.Handler.Logout(w, r)
	if w.err != nil {
		return nil, w.err
	}
	return &pb.LogoutResponse{Status: w.status}, nil
}

type nopCloser struct {
	b   []byte
	off int
}

func newNopCloser(v interface{}) nopCloser {
	b, _ := json.Marshal(v)
	return nopCloser{b: b}
}

func (n *nopCloser) Read(p []byte) (int, error) {
	if n.off >= len(n.b) {
		return 0, io.EOF
	}
	c := copy(p, n.b[n.off:])
	n.off += c
	return c, nil
}
func (n *nopCloser) Close() error { return nil }

// To adapt to HTTP response

type responseWriter struct {
	id  int64
	err error
}

func (w *responseWriter) Header() http.Header { return http.Header{} }
func (w *responseWriter) Write(b []byte) (int, error) {
	var resp struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(b, &resp); err != nil {
		w.err = err
		return 0, err
	}
	w.id = resp.ID
	return len(b), nil
}
func (w *responseWriter) WriteHeader(statusCode int) {}

// Token writer

type respTokenWriter struct {
	access, refresh string
	err             error
}

func (w *respTokenWriter) Header() http.Header { return http.Header{} }
func (w *respTokenWriter) Write(b []byte) (int, error) {
	var resp struct{ Access, Refresh string }
	if err := json.Unmarshal(b, &resp); err != nil {
		w.err = err
		return 0, err
	}
	w.access = resp.Access
	w.refresh = resp.Refresh
	return len(b), nil
}
func (w *respTokenWriter) WriteHeader(statusCode int) {}

// Status writer

type respStatusWriter struct {
	status string
	err    error
}

func (w *respStatusWriter) Header() http.Header { return http.Header{} }
func (w *respStatusWriter) Write(b []byte) (int, error) {
	var resp struct{ Status string }
	if err := json.Unmarshal(b, &resp); err != nil {
		w.err = err
		return 0, err
	}
	w.status = resp.Status
	return len(b), nil
}
func (w *respStatusWriter) WriteHeader(statusCode int) {}
