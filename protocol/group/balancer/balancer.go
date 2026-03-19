package balancer

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/protocol/group/healthcheck"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.InterfaceUpdateListener = (*Balancer)(nil)

// Balancer is the load balancer
type Balancer struct {
	*healthcheck.HealthCheck

	providers []adapter.Provider
	logger    log.ContextLogger
	cfg       balancerConfig

	objective Objective
	strategy  Strategy

	networks []string
}

// New creates a new load balancer
//
// The globalHistory is optional and is only used to sync latency history
// between different health checkers. Each HealthCheck will maintain its own
// history storage since different ones can have different check destinations,
// sampling numbers, etc.
func New(
	ctx context.Context,
	router adapter.Router,
	outbound adapter.OutboundManager,
	providers []adapter.Provider,
	options *option.LoadBalanceOutboundOptions, logger log.ContextLogger,
) (*Balancer, error) {
	if options == nil {
		options = &option.LoadBalanceOutboundOptions{}
	}
	cfg, err := configFromOptions(*options)
	if err != nil {
		return nil, err
	}
	var (
		objective Objective
		strategy  Strategy
	)
	switch cfg.Pick.Objective {
	case ObjectiveAlive:
		objective = NewAliveObjective()
	case ObjectiveQualified:
		objective = NewQualifiedObjective()
	case ObjectiveLeastLoad:
		objective = NewLeastLoadObjective(options.Pick)
	case ObjectiveLeastPing:
		objective = NewLeastPingObjective(options.Pick)
	default:
		return nil, E.New("unknown objective: ", cfg.Pick.Objective)
	}
	switch cfg.Pick.Strategy {
	case StrategyRandom:
		strategy = NewRandomStrategy()
	case StrategyRoundrobin:
		strategy = NewRoundRobinStrategy()
	case StrategyConsistentHash:
		if cfg.Pick.Objective != ObjectiveAlive {
			return nil, E.New("consistenthash strategy works only with 'alive' objective")
		}
		strategy = NewConsistentHashStrategy()
	default:
		return nil, E.New("unknown strategy: ", cfg.Pick.Strategy)
	}

	// healthcheck.New() may apply default values to options, e.g. the `sampling` which
	// is used to calculate the maxFailRate.
	hc := healthcheck.New(ctx, router, outbound, providers, &options.Check, logger)

	return &Balancer{
		cfg:         cfg,
		logger:      logger,
		providers:   providers,
		HealthCheck: hc,
		objective:   objective,
		strategy:    strategy,
	}, nil
}

// Pick picks a node
func (b *Balancer) Pick(ctx context.Context, network string, destination M.Socksaddr) adapter.Outbound {
	metadata := adapter.ContextFrom(ctx)
	if metadata == nil {
		metadata = &adapter.InboundContext{}
	}
	metadata.Destination = destination
	all := b.Nodes(network)
	filtered := b.objective.Filter(all)
	picked := b.strategy.Pick(all, filtered, metadata)
	if picked == nil {
		return nil
	}
	return picked.Outbound
}

// Networks returns all networks supported by this balancer
func (b *Balancer) Networks() []string {
	if b.networks == nil {
		b.networks = b.availableNetworks()
	}
	return b.networks
}

// Nodes returns all Nodes for the network
func (b *Balancer) Nodes(network string) []*Node {
	all := make([]*Node, 0)
	idx := 0
	for _, provider := range b.providers {
		for _, outbound := range provider.Outbounds() {
			idx++
			networks := outbound.Network()
			if network != "" && !common.Contains(networks, network) {
				continue
			}
			if group, ok := outbound.(adapter.OutboundGroup); ok {
				real, err := adapter.RealOutbound(group)
				if err != nil {
					continue
				}
				outbound = real
			}
			scale := calcFactor(outbound.Tag(), b.cfg.pickBiases)
			stats := b.HealthCheck.Storage.Stats(outbound.Tag())
			status := calcStatus(&stats, b.cfg.maxRTT, b.cfg.maxFailRate)
			node := NewNode(outbound, idx, scale, stats, status)
			all = append(all, node)
		}
	}
	return all
}

// availableNetworks returns available networks of qualified nodes
func (b *Balancer) availableNetworks() []string {
	var hasTCP, hasUDP bool
	nodes := b.Nodes("")
	for _, n := range nodes {
		if !hasTCP && common.Contains(n.Network(), N.NetworkTCP) {
			hasTCP = true
		}
		if !hasUDP && common.Contains(n.Network(), N.NetworkUDP) {
			hasUDP = true
		}
		if hasTCP && hasUDP {
			break
		}
	}
	switch {
	case hasTCP && hasUDP:
		return []string{N.NetworkTCP, N.NetworkUDP}
	case hasTCP:
		return []string{N.NetworkTCP}
	case hasUDP:
		return []string{N.NetworkUDP}
	default:
		return []string{}
	}
}

// LogNodesAndReturn logs all nodes status and returns the list of nodes sorted by the objective.
// The available nodes are in front of the slice, and the unavailable nodes are in the back.
func (b *Balancer) LogNodesAndReturn() (all []*Node, available int) {
	all = b.Nodes("")
	filtered := b.objective.Filter(all)
	available = len(filtered)
	b.logger.Info(
		b.cfg.Pick.Objective, "/", b.cfg.Pick.Strategy, ", ",
		available, " of ", len(all), " nodes available",
	)
	b.logger.Info("=== nodes available ===")
	b.objective.Sort(all)
	for i, n := range all {
		if i == available {
			b.logger.Info("=== nodes unavailable ===")
		}
		b.logger.Info(n.String())
	}
	return all, available
}

// InterfaceUpdated implements adapter.InterfaceUpdateListener
func (b *Balancer) InterfaceUpdated() {
	// b can be nil if the parent struct has not initialized it yet.
	if b == nil || b.HealthCheck == nil {
		return
	}
	go b.HealthCheck.CheckAll(context.Background())
}
