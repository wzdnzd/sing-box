package balancer

import (
	"regexp"
	"time"

	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/protocol/group/healthcheck"
	"github.com/sagernet/sing/common/json/badoption"
)

type balancerConfig struct {
	option.LoadBalanceOutboundOptions
	maxRTT      healthcheck.RTT
	maxFailRate float32
	pickBiases  []pickBias
}

type pickBias struct {
	option.LoadBalancePickBias
	Regexp *regexp.Regexp
}

func configFromOptions(options option.LoadBalanceOutboundOptions) (balancerConfig, error) {
	if options.Pick.Strategy == "" {
		options.Pick.Strategy = StrategyRandom
	}
	if options.Pick.Objective == "" {
		options.Pick.Objective = ObjectiveAlive
	}
	if options.Check.Interval <= 0 {
		options.Check.Interval = badoption.Duration(5 * time.Minute)
	}

	var cfg balancerConfig
	cfg.LoadBalanceOutboundOptions = options
	if options.Check.Sampling > 0 {
		cfg.maxFailRate = float32(options.Pick.MaxFail) / float32(options.Check.Sampling)
	}
	cfg.maxRTT = healthcheck.RTT(options.Pick.MaxRTT.Build().Milliseconds())

	for _, bias := range options.Pick.Biases {
		var re *regexp.Regexp
		if bias.Regexp != "" {
			var err error
			re, err = regexp.Compile(bias.Regexp)
			if err != nil {
				return cfg, err
			}
		}
		cfg.pickBiases = append(cfg.pickBiases, pickBias{
			LoadBalancePickBias: bias,
			Regexp:              re,
		})
	}
	return cfg, nil
}
