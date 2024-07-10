package nethttp

import (
	"fmt"
	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/config"
	"github.com/uber/jaeger-client-go/zipkin"
)

type (
	OptionsTrace struct {
		enableB3Extractor         bool
		enableB3Injector          bool
		enableZipkinSharedRPCSpan bool
		config                    *config.Configuration
	}

	OptionTrace func(opt *OptionsTrace)
)

func EnableB3Extractor(enable bool) OptionTrace {
	return func(opt *OptionsTrace) {
		opt.enableB3Extractor = enable
	}
}

func EnableB3Injector(enable bool) OptionTrace {
	return func(opt *OptionsTrace) {
		opt.enableB3Injector = enable
	}
}

func EnableLogSpans(enable bool) OptionTrace {
	return func(opt *OptionsTrace) {
		opt.config.Reporter.LogSpans = enable
	}
}

func ServiceName(name string) OptionTrace {
	return func(opt *OptionsTrace) {
		opt.config.ServiceName = name
	}
}

func NewTraceClient(opts ...OptionTrace) opentracing.Tracer {
	env, err := config.FromEnv()
	if err != nil {
		return nil
	}

	optTrace := &OptionsTrace{
		config: env,
	}

	for _, opt := range opts {
		opt(optTrace)
	}

	optsCfg := []config.Option{
		config.Logger(jaeger.StdLogger),
		config.MaxTagValueLength(256),
		config.PoolSpans(true),
		config.Gen128Bit(true),
	}

	if optTrace.enableB3Extractor || optTrace.enableB3Injector {
		propagator := zipkin.NewZipkinB3HTTPHeaderPropagator()

		if optTrace.enableB3Injector {
			optsCfg = append(optsCfg, config.Injector(opentracing.HTTPHeaders, propagator))
		}

		if optTrace.enableB3Extractor {
			optsCfg = append(optsCfg, config.Extractor(opentracing.HTTPHeaders, propagator))
		}
	}

	if optTrace.enableZipkinSharedRPCSpan {
		optsCfg = append(optsCfg, config.ZipkinSharedRPCSpan(true))
	}

	trace, _, err := optTrace.config.NewTracer(optsCfg...)
	if err != nil {
		panic(fmt.Sprintf("error: cannot init jaeger: %v\n", err))
	}

	opentracing.SetGlobalTracer(trace)
	return trace
}
