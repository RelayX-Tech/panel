// Package registration 实现 RegistrationService 的 gRPC 适配层（design §3.10）。
package registration

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"os"

	agentv1 "github.com/relayx-tech/contracts/gen/go/agent/v1"
	"github.com/relayx-tech/panel/internal/config"
	"github.com/relayx-tech/panel/internal/logic/register"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// NewGRPCServer 构造注册 gRPC 服务端；Insecure=false 时启用 server-auth TLS
// （RequestClientCert：Register 无需客户端证书，Renew 有则校验并取 CN）。
func NewGRPCServer(cfg config.RegistrationConf, svc *register.Service) (*grpc.Server, error) {
	var opts []grpc.ServerOption
	if !cfg.Insecure {
		tlsCfg, err := registrationTLSConfig(cfg)
		if err != nil {
			return nil, err
		}
		opts = append(opts, grpc.Creds(credentials.NewTLS(tlsCfg)))
	}
	server := grpc.NewServer(opts...)
	agentv1.RegisterRegistrationServiceServer(server, New(svc))
	return server, nil
}

func registrationTLSConfig(cfg config.RegistrationConf) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if cfg.CAFile != "" {
		caPEM, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, err
		}
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, errors.New("客户端 CA PEM 解析失败")
		}
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    pool,
		ClientAuth:   tls.RequestClientCert,
		MinVersion:   tls.VersionTLS13,
	}, nil
}

// Server 是 RegistrationService 的 gRPC 实现。
type Server struct {
	agentv1.UnimplementedRegistrationServiceServer
	svc *register.Service
}

// New 构造适配器。
func New(svc *register.Service) *Server { return &Server{svc: svc} }

// Register 预认证注册（server-auth TLS + 一次性 Token）。
func (s *Server) Register(ctx context.Context, req *agentv1.RegisterRequest) (*agentv1.RegisterResponse, error) {
	res, err := s.svc.Register(ctx, req.GetToken(), req.GetNodeId(), nodeTypeName(req.GetNodeType()), req.GetCsrPem())
	if err != nil {
		return nil, mapError(err)
	}
	return toResponse(res), nil
}

// Renew 已认证续期（节点标识取自 mTLS 客户端证书 CN）。
func (s *Server) Renew(ctx context.Context, req *agentv1.RenewRequest) (*agentv1.RegisterResponse, error) {
	nodeID, err := nodeIDFromPeer(ctx)
	if err != nil {
		return nil, err
	}
	res, rerr := s.svc.Renew(ctx, nodeID, req.GetCsrPem())
	if rerr != nil {
		return nil, mapError(rerr)
	}
	return toResponse(res), nil
}

// GetCA 返回 CA 证书 PEM。
func (s *Server) GetCA(context.Context, *agentv1.GetCARequest) (*agentv1.GetCAResponse, error) {
	return &agentv1.GetCAResponse{CaPem: s.svc.GetCA()}, nil
}

func toResponse(res *register.Result) *agentv1.RegisterResponse {
	return &agentv1.RegisterResponse{
		CertChainPem:                res.CertChainPEM,
		CaPem:                       res.CaPEM,
		NotAfter:                    timestamppb.New(res.NotAfter),
		PinnedCertFingerprintSha256: res.Fingerprint,
	}
}

func nodeTypeName(t agentv1.AgentNodeType) string {
	switch t {
	case agentv1.AgentNodeType_AGENT_NODE_TYPE_ENTRY:
		return "entry"
	case agentv1.AgentNodeType_AGENT_NODE_TYPE_TUNNEL:
		return "tunnel"
	default:
		return ""
	}
}

func nodeIDFromPeer(ctx context.Context) (string, error) {
	p, ok := peer.FromContext(ctx)
	if !ok || p.AuthInfo == nil {
		return "", status.Error(codes.Unauthenticated, "缺少客户端证书")
	}
	info, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok || len(info.State.PeerCertificates) == 0 {
		return "", status.Error(codes.Unauthenticated, "缺少客户端证书")
	}
	cn := info.State.PeerCertificates[0].Subject.CommonName
	if cn == "" {
		return "", status.Error(codes.Unauthenticated, "客户端证书缺少 CN")
	}
	return cn, nil
}

func mapError(err error) error {
	switch {
	case errors.Is(err, register.ErrTokenInvalid):
		return detailError(codes.Unauthenticated, "AUTH_TOKEN_INVALID", err)
	case errors.Is(err, register.ErrTokenExpired):
		return detailError(codes.Unauthenticated, "AUTH_TOKEN_EXPIRED", err)
	case errors.Is(err, register.ErrTokenUsed):
		return detailError(codes.Unauthenticated, "AUTH_TOKEN_USED", err)
	case errors.Is(err, register.ErrNodeTypeMismatch):
		return detailError(codes.FailedPrecondition, "NODE_TYPE_MISMATCH", err)
	case errors.Is(err, register.ErrAlreadyRegistered):
		return detailError(codes.AlreadyExists, "NODE_ALREADY_REGISTERED", err)
	case errors.Is(err, register.ErrCSRInvalid):
		return detailError(codes.InvalidArgument, "CSR_INVALID", err)
	case errors.Is(err, register.ErrCertIssue):
		return detailError(codes.Internal, "CERT_ISSUE_FAILED", err)
	case errors.Is(err, register.ErrCertRevoked):
		return detailError(codes.PermissionDenied, "AUTH_CERT_REVOKED", err)
	case errors.Is(err, register.ErrNodeNotFound):
		return detailError(codes.NotFound, "NODE_NOT_FOUND", err)
	default:
		return detailError(codes.Internal, "INTERNAL", err)
	}
}

func detailError(c codes.Code, code string, err error) error {
	st := status.New(c, err.Error())
	withDetails, derr := st.WithDetails(&agentv1.AgentErrorDetail{Code: code, Message: err.Error()})
	if derr != nil {
		return st.Err()
	}
	return withDetails.Err()
}
