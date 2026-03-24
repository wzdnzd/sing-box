package outbound

import (
	"context"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
)

type ConstructorFunc[T any] func(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options T) (adapter.Outbound, error)

// DeriverFunc is the function type for deriving additional outbound options from an outbound option.
// The derived option.Outbound.Options should be a pointer type, and the function can return nil if no options are derived.
type DeriverFunc[T any] func(tag string, options T) []option.Outbound

func Register[Options any](registry *Registry, outboundType string, constructor ConstructorFunc[Options], deriver ...DeriverFunc[Options]) {
	registry.register(outboundType, func() any {
		return new(Options)
	}, func(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, rawOptions any) (adapter.Outbound, error) {
		var options *Options
		if rawOptions != nil {
			options = rawOptions.(*Options)
		}
		return constructor(ctx, router, logger, tag, common.PtrValueOrDefault(options))
	}, func(tag string, rawOptions any) []option.Outbound {
		if len(deriver) == 0 {
			return nil
		}
		var options *Options
		if rawOptions != nil {
			options = rawOptions.(*Options)
		}
		return deriver[0](tag, common.PtrValueOrDefault(options))
	})
}

var _ adapter.OutboundRegistry = (*Registry)(nil)

type (
	optionsConstructorFunc func() any
	constructorFunc        func(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options any) (adapter.Outbound, error)
	deriverFunc            func(tag string, options any) []option.Outbound
)
type Registry struct {
	access       sync.Mutex
	optionsType  map[string]optionsConstructorFunc
	constructors map[string]constructorFunc

	derivers map[string]deriverFunc
}

func NewRegistry() *Registry {
	return &Registry{
		optionsType:  make(map[string]optionsConstructorFunc),
		constructors: make(map[string]constructorFunc),
		derivers:     make(map[string]deriverFunc),
	}
}

func (r *Registry) CreateOptions(outboundType string) (any, bool) {
	r.access.Lock()
	defer r.access.Unlock()
	optionsConstructor, loaded := r.optionsType[outboundType]
	if !loaded {
		return nil, false
	}
	return optionsConstructor(), true
}

func (r *Registry) DeriveOptions(outboundType string, tag string, options any) []option.Outbound {
	r.access.Lock()
	defer r.access.Unlock()
	deriver, loaded := r.derivers[outboundType]
	if !loaded {
		return nil
	}
	return deriver(tag, options)
}

func (r *Registry) CreateOutbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, outboundType string, options any) (adapter.Outbound, error) {
	r.access.Lock()
	defer r.access.Unlock()
	constructor, loaded := r.constructors[outboundType]
	if !loaded {
		return nil, E.New("outbound type not found: " + outboundType)
	}
	return constructor(ctx, router, logger, tag, options)
}

func (r *Registry) register(outboundType string, optionsConstructor optionsConstructorFunc, constructor constructorFunc, deriver deriverFunc) {
	r.access.Lock()
	defer r.access.Unlock()
	r.optionsType[outboundType] = optionsConstructor
	r.constructors[outboundType] = constructor
	r.derivers[outboundType] = deriver
}
