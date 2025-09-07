package grpcapi

import (
	"context"
	"net"
	"strings"
	"sync"

	"transaction-service/internal/db"
	"transaction-service/internal/models"
	transactionsv1 "transaction-service/proto"

	"golang.org/x/time/rate"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Server implements transactions.v1.Transactions.
type Server struct {
	transactionsv1.UnimplementedTransactionsServer
	Repo *db.TransactionRepo

	// simple in-memory rate limiting (per-IP and per-user)
	mu          sync.Mutex
	ipLimiter   map[string]*rate.Limiter
	userLimiter map[string]*rate.Limiter
	ipRate      rate.Limit
	userRate    rate.Limit
	burst       int
}

func NewServer(repo *db.TransactionRepo, ipPerMin, userPerMin int) *Server {
	return &Server{
		Repo:        repo,
		ipLimiter:   map[string]*rate.Limiter{},
		userLimiter: map[string]*rate.Limiter{},
		ipRate:      rate.Limit(float64(ipPerMin) / 60.0),
		userRate:    rate.Limit(float64(userPerMin) / 60.0),
		burst:       20,
	}
}

func (s *Server) getLimiter(m map[string]*rate.Limiter, key string, r rate.Limit) *rate.Limiter {
	s.mu.Lock()
	defer s.mu.Unlock()
	if lim, ok := m[key]; ok {
		return lim
	}
	lim := rate.NewLimiter(r, s.burst)
	m[key] = lim
	return lim
}

func (s *Server) allow(ctx context.Context) bool {
	// Derive client IP from peer info
	ip := "unknown"
	if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
		ip = p.Addr.String()
		if host, _, err := net.SplitHostPort(ip); err == nil {
			ip = host
		}
	}
	ipLimiter := s.getLimiter(s.ipLimiter, ip, s.ipRate)
	if !ipLimiter.Allow() {
		return false
	}

	// Optional user limiter via metadata "x-user-id"
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		vals := md.Get("x-user-id")
		if len(vals) > 0 {
			uid := strings.TrimSpace(vals[0])
			if uid != "" {
				userLimiter := s.getLimiter(s.userLimiter, uid, s.userRate)
				if !userLimiter.Allow() {
					return false
				}
			}
		}
	}
	return true
}

func (s *Server) CreateTransaction(ctx context.Context, req *transactionsv1.CreateTransactionRequest) (*transactionsv1.Transaction, error) {
	if !s.allow(ctx) {
		return nil, status.Errorf(codes.ResourceExhausted, "rate limit exceeded")
	}

	// Basic validation
	if strings.TrimSpace(req.GetCoinid()) == "" ||
		strings.TrimSpace(req.GetUserid()) == "" ||
		strings.TrimSpace(req.GetDataid()) == "" ||
		strings.TrimSpace(req.GetPlatformName()) == "" {
		return nil, status.Errorf(codes.InvalidArgument, "coinid, userid, dataid, and platform_name are required")
	}
	if req.GetCoinused() < 0 {
		return nil, status.Errorf(codes.InvalidArgument, "coinused must be non-negative")
	}
	txTS := req.GetTransactionTimestamp()
	expTS := req.GetExpiryDate()
	if txTS == nil || expTS == nil {
		return nil, status.Errorf(codes.InvalidArgument, "transaction_timestamp and expiry_date are required")
	}
	txTime := txTS.AsTime().UTC()
	expTime := expTS.AsTime().UTC()
	if expTime.Before(txTime) {
		return nil, status.Errorf(codes.InvalidArgument, "expiry_date must be >= transaction_timestamp")
	}

	// Convert request to model
	model := models.Transaction{
		CoinID:               req.GetCoinid(),
		UserID:               req.GetUserid(),
		DataID:               req.GetDataid(),
		CoinUsed:             req.GetCoinused(),
		TransactionTimestamp: txTime,
		ExpiryDate:           expTime,
		PlatformName:         req.GetPlatformName(),
	}

	out, err := s.Repo.Insert(ctx, model)
	if err != nil {
		// Map DB errors as needed (e.g., unique violation -> AlreadyExists)
		return nil, status.Errorf(codes.Internal, "insert failed: %v", err)
	}

	return &transactionsv1.Transaction{
		Id:                   out.ID,
		Coinid:               out.CoinID,
		Userid:               out.UserID,
		Dataid:               out.DataID,
		Coinused:             out.CoinUsed,
		TransactionTimestamp: timestamppb.New(out.TransactionTimestamp.UTC()),
		ExpiryDate:           timestamppb.New(out.ExpiryDate.UTC()),
		PlatformName:         out.PlatformName,
	}, nil
}
