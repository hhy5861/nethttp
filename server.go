package nethttp

import (
	"github.com/gin-gonic/gin"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"net/http"
	"net/url"
)

var responseSizeKey = "http.response_size"

type Options struct {
	opNameFunc    func(r *http.Request) string
	spanFilter    func(r *http.Request) bool
	spanObserver  func(span opentracing.Span, r *http.Request)
	urlTagFunc    func(u *url.URL) string
	componentName string
	tracer        opentracing.Tracer
}

type Option func(opt *Options)

func OperationNameFunc(f func(r *http.Request) string) Option {
	return func(options *Options) {
		options.opNameFunc = f
	}
}

func MWComponentName(componentName string) Option {
	return func(options *Options) {
		options.componentName = componentName
	}
}

func MWSpanFilter(f func(r *http.Request) bool) Option {
	return func(options *Options) {
		options.spanFilter = f
	}
}

func MWSpanObserver(f func(span opentracing.Span, r *http.Request)) Option {
	return func(options *Options) {
		options.spanObserver = f
	}
}

func MWURLTagFunc(f func(u *url.URL) string) Option {
	return func(options *Options) {
		options.urlTagFunc = f
	}
}

func NewServerTrace(trace opentracing.Tracer) Option {
	return func(opt *Options) {
		opt.tracer = trace
	}
}

func NewTracerServer(opt ...Option) *Options {
	opts := &Options{
		opNameFunc: func(r *http.Request) string {
			return "HTTP " + r.Method + ": " + r.URL.Path
		},
		spanFilter:   func(r *http.Request) bool { return true },
		spanObserver: func(span opentracing.Span, r *http.Request) {},
		urlTagFunc: func(u *url.URL) string {
			return u.String()
		},
		tracer:        opentracing.GlobalTracer(),
		componentName: defaultComponentName,
	}

	for _, o := range opt {
		o(opts)
	}

	return opts
}

func (opts *Options) MiddlewareWithGinFunc(c *gin.Context) {
	if !opts.spanFilter(c.Request) {
		c.Next()

		return
	}

	ctx, _ := opts.tracer.Extract(opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(c.Request.Header))
	sp := opts.tracer.StartSpan(opts.opNameFunc(c.Request), ext.RPCServerOption(ctx))

	ext.HTTPMethod.Set(sp, c.Request.Method)
	ext.HTTPUrl.Set(sp, opts.urlTagFunc(c.Request.URL))
	ext.Component.Set(sp, opts.componentName)
	opts.spanObserver(sp, c.Request)

	mt := &metricsTracker{ResponseWriter: c.Writer}
	c.Request = c.Request.WithContext(opentracing.ContextWithSpan(c.Request.Context(), sp))

	defer func() {
		panicErr := recover()
		didPanic := panicErr != nil

		if mt.status == 0 && !didPanic {
			mt.status = 200
		}

		if mt.status > 0 {
			ext.HTTPStatusCode.Set(sp, uint16(mt.status))
		}

		if mt.size > 0 {
			sp.SetTag(responseSizeKey, mt.size)
		}

		if mt.status >= http.StatusInternalServerError || didPanic {
			ext.Error.Set(sp, true)
		}

		sp.Finish()

		if didPanic {
			panic(panicErr)
		}

		mt.wrappedResponseWriter()
	}()

	c.Next()
}

func (opts *Options) Middleware(tr opentracing.Tracer, h http.Handler, options ...Option) http.Handler {
	return opts.MiddlewareFunc(tr, h.ServeHTTP, options...)
}

func (opts *Options) MiddlewareFunc(tr opentracing.Tracer, h http.HandlerFunc, options ...Option) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		if !opts.spanFilter(r) {
			h(w, r)
			return
		}

		ctx, _ := tr.Extract(opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(r.Header))
		sp := tr.StartSpan(opts.opNameFunc(r), ext.RPCServerOption(ctx))

		ext.HTTPMethod.Set(sp, r.Method)
		ext.HTTPUrl.Set(sp, opts.urlTagFunc(r.URL))
		ext.Component.Set(sp, opts.componentName)

		opts.spanObserver(sp, r)

		mt := &metricsTracker{ResponseWriter: w}
		r = r.WithContext(opentracing.ContextWithSpan(r.Context(), sp))

		defer func() {
			panicErr := recover()
			didPanic := panicErr != nil

			if mt.status == 0 && !didPanic {
				mt.status = 200
			}

			if mt.status > 0 {
				ext.HTTPStatusCode.Set(sp, uint16(mt.status))
			}

			if mt.size > 0 {
				sp.SetTag(responseSizeKey, mt.size)
			}

			if mt.status >= http.StatusInternalServerError || didPanic {
				ext.Error.Set(sp, true)
			}

			sp.Finish()

			if didPanic {
				panic(panicErr)
			}
		}()

		h(mt.wrappedResponseWriter(), r)
	}

	return fn
}
