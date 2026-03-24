package group

import (
	"context"
	"net"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/interrupt"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/protocol/group/balancer"
	"github.com/sagernet/sing-box/provider"
	tun "github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
)

func RegisterLoadBalanceProfile(registry *outbound.Registry) {
	outbound.Register(registry, C.TypeLoadBalanceProfile, NewLoadBalanceProfile)
}

var (
	_ adapter.Outbound                = (*LoadBalanceProfile)(nil)
	_ adapter.OutboundCheckGroup      = (*LoadBalanceProfile)(nil)
	_ adapter.DirectRouteOutbound     = (*LoadBalanceProfile)(nil)
	_ adapter.SimpleLifecycle         = (*LoadBalanceProfile)(nil)
	_ adapter.InterfaceUpdateListener = (*LoadBalanceProfile)(nil)
)

// LoadBalanceProfile is a load balance group
type LoadBalanceProfile struct {
	profileAdapter
	*balancer.Balancer

	ctx        context.Context
	logger     log.ContextLogger
	outbound   adapter.OutboundManager
	connection adapter.ConnectionManager
	options    option.LoadBalanceProfileOutboundOptions
}

// NewLoadBalanceProfile creates a new load balance outbound
func NewLoadBalanceProfile(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.LoadBalanceProfileOutboundOptions) (adapter.Outbound, error) {
	return &LoadBalanceProfile{
		profileAdapter: *newProfileAdapter(
			C.TypeLoadBalanceProfile, tag, options.Exclude, options.Include,
			[]string{options.LoadBalanceTag},
		),
		ctx:        ctx,
		logger:     logger,
		outbound:   service.FromContext[adapter.OutboundManager](ctx),
		connection: service.FromContext[adapter.ConnectionManager](ctx),
		options:    options,
	}, nil
}

// Now implements adapter.OutboundGroup
func (s *LoadBalanceProfile) Now() string {
	picked := s.Pick(context.Background(), N.NetworkTCP, M.Socksaddr{})
	if picked == nil {
		return ""
	}
	return picked.Tag()
}

// All implements adapter.OutboundGroup
func (s *LoadBalanceProfile) All() []string {
	_, filtered := s.GetNodes(false)
	return common.Map(filtered, func(node *balancer.Node) string {
		return node.Tag()
	})
}

// Network implements adapter.OutboundGroup
func (s *LoadBalanceProfile) Network() []string {
	return s.Balancer.Networks()
}

// DialContext implements adapter.Outbound
func (s *LoadBalanceProfile) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	var lastErr error
	maxRetry := 5
	for i := 0; i < maxRetry; i++ {
		picked := s.Pick(ctx, network, destination)
		if picked == nil {
			lastErr = E.New("no outbound available")
			break
		}
		conn, err := picked.DialContext(ctx, network, destination)
		if err == nil {
			return conn, nil
		}
		lastErr = err
		s.logger.ErrorContext(ctx, err)
		s.ReportFailure(picked)
	}
	return nil, lastErr
}

// ListenPacket implements adapter.Outbound
func (s *LoadBalanceProfile) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	var lastErr error
	maxRetry := 5
	for i := 0; i < maxRetry; i++ {
		picked := s.Pick(ctx, N.NetworkUDP, destination)
		if picked == nil {
			lastErr = E.New("no outbound available")
			break
		}
		conn, err := picked.ListenPacket(ctx, destination)
		if err == nil {
			return conn, nil
		}
		lastErr = err
		s.logger.ErrorContext(ctx, err)
		s.ReportFailure(picked)
	}
	return nil, lastErr
}

// NewConnectionEx implements adapter.TCPInjectableInbound
func (s *LoadBalanceProfile) NewConnectionEx(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	selected := s.Pick(ctx, N.NetworkUDP, metadata.Destination)
	if selected == nil {
		s.connection.NewConnection(ctx, newErrDailer(E.New("no outbound available")), conn, metadata, onClose)
		return
	}
	ctx = interrupt.ContextWithIsExternalConnection(ctx)
	if outboundHandler, isHandler := selected.(adapter.ConnectionHandlerEx); isHandler {
		outboundHandler.NewConnectionEx(ctx, conn, metadata, onClose)
	} else {
		s.connection.NewConnection(ctx, selected, conn, metadata, onClose)
	}
}

// NewPacketConnectionEx implements adapter.UDPInjectableInbound
func (s *LoadBalanceProfile) NewPacketConnectionEx(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	selected := s.Pick(ctx, N.NetworkUDP, metadata.Destination)
	if selected == nil {
		s.connection.NewPacketConnection(ctx, newErrDailer(E.New("no outbound available")), conn, metadata, onClose)
		return
	}
	ctx = interrupt.ContextWithIsExternalConnection(ctx)
	if outboundHandler, isHandler := selected.(adapter.PacketConnectionHandlerEx); isHandler {
		outboundHandler.NewPacketConnectionEx(ctx, conn, metadata, onClose)
	} else {
		s.connection.NewPacketConnection(ctx, selected, conn, metadata, onClose)
	}
}

// NewDirectRouteConnection implements adapter.DirectRouteOutbound
func (s *LoadBalanceProfile) NewDirectRouteConnection(metadata adapter.InboundContext, routeContext tun.DirectRouteContext, timeout time.Duration) (tun.DirectRouteDestination, error) {
	ctx := adapter.WithContext(context.Background(), &metadata)
	destination := metadata.Destination
	picked := s.Pick(ctx, N.NetworkICMP, destination)
	if picked == nil {
		return nil, E.New("no outbound available for network: ", metadata.Network)
	}
	if !common.Contains(picked.Network(), metadata.Network) {
		return nil, E.New(metadata.Network, " is not supported by outbound: ", picked.Tag())
	}
	dro, ok := picked.(adapter.DirectRouteOutbound)
	if !ok {
		return nil, E.New("outbound does not support direct route: ", picked.Tag())
	}
	return dro.NewDirectRouteConnection(metadata, routeContext, timeout)
}

// Close implements adapter.Service
func (s *LoadBalanceProfile) Close() error {
	if s.Balancer == nil {
		return nil
	}
	return s.Balancer.Close()
}

// Start implements adapter.Service
func (s *LoadBalanceProfile) Start() error {
	outbound, ok := s.outbound.Outbound(s.options.LoadBalanceTag)
	if !ok {
		return E.New("loadbalance not found: ", s.options.LoadBalanceTag)
	}
	lb, ok := outbound.(*LoadBalance)
	if !ok {
		return E.New("outbound is not a load balance: ", s.options.LoadBalanceTag)
	}
	s.profileAdapter.SetUpstream(&lb.GroupAdapter)
	b, err := balancer.New(s.logger, &s.profileAdapter, lb.HealthCheck, s.options.Pick)
	if err != nil {
		return err
	}
	s.Balancer = b
	return s.Balancer.Start()
}

type profileAdapter struct {
	typ     string
	tag     string
	exclude string
	include string
	deps    []string
	network []string

	providers      []adapter.Provider
	providersByTag map[string]adapter.Provider
}

func newProfileAdapter(typ string, tag string, exclude, include string, deps []string) *profileAdapter {
	return &profileAdapter{
		typ:     typ,
		tag:     tag,
		deps:    deps,
		exclude: exclude,
		include: include,
	}
}

func (a *profileAdapter) Type() string {
	return a.typ
}

func (a *profileAdapter) Tag() string {
	return a.tag
}

func (a *profileAdapter) Network() []string {
	return a.network
}

func (a *profileAdapter) Dependencies() []string {
	return a.deps
}
func (a *profileAdapter) Provider(tag string) (adapter.Provider, bool) {
	if a.providersByTag == nil {
		return nil, false
	}
	p, ok := a.providersByTag[tag]
	return p, ok
}

func (a *profileAdapter) Providers() []adapter.Provider {
	return a.providers
}

func (a *profileAdapter) Outbound(tag string) (adapter.Outbound, bool) {
	if len(a.providers) == 0 {
		return nil, false
	}
	for _, p := range a.providers {
		if outbound, ok := p.Outbound(tag); ok {
			return outbound, true
		}
	}
	return nil, false
}

func (a *profileAdapter) Outbounds() []adapter.Outbound {
	if len(a.providers) == 0 {
		return nil
	}
	var outbounds []adapter.Outbound
	for _, p := range a.providers {
		outbounds = append(outbounds, p.Outbounds()...)
	}
	return outbounds
}

func (a *profileAdapter) SetUpstream(upstream *outbound.GroupAdapter) {
	a.network = upstream.Network()
	a.providers = common.Map(upstream.Providers(), func(p adapter.Provider) adapter.Provider {
		if a.exclude == "" && a.include == "" {
			return p
		}
		r, err := provider.NewFiltered(p, a.exclude, a.include)
		if err != nil {
			log.Error("[", a.tag, "] failed to create filtered provider: ", err)
			return p
		}
		return r
	})
	a.providersByTag = make(map[string]adapter.Provider)
	for _, p := range a.providers {
		a.providersByTag[p.Tag()] = p
	}
}
