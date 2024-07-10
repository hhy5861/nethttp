package nethttp

import (
	"context"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"sync"
)

type contextKey struct{}

var (
	keyTracer = contextKey{}
)

const defaultComponentName = "net/http"

type Transport struct {
	http.RoundTripper
}

func TraceRequest(tr opentracing.Tracer, req *http.Request, options ...ClientOption) (*http.Request, *Tracer) {
	opts := &clientOptions{
		urlTagFunc: func(u *url.URL) string {
			return u.String()
		},
		spanObserver:  func(_ opentracing.Span, _ *http.Request) {},
		componentName: defaultComponentName,
	}

	for _, opt := range options {
		opt(opts)
	}

	ht := &Tracer{tr: tr, opts: opts}

	ctx := req.Context()
	if !opts.disableClientTrace {
		ctx = httptrace.WithClientTrace(ctx, ht.clientTrace())
	}

	req = req.WithContext(context.WithValue(ctx, keyTracer, ht))
	return req, ht
}

func TraceWithContext(tr opentracing.Tracer, ctx context.Context, options ...ClientOption) context.Context {
	opts := &clientOptions{
		urlTagFunc: func(u *url.URL) string {
			return u.String()
		},
		spanObserver:  func(_ opentracing.Span, _ *http.Request) {},
		componentName: defaultComponentName,
	}

	for _, opt := range options {
		opt(opts)
	}

	ht := &Tracer{tr: tr, opts: opts}

	if !opts.disableClientTrace {
		ctx = httptrace.WithClientTrace(ctx, ht.clientTrace())
	}

	return context.WithValue(ctx, keyTracer, ht)
}

func TracerFromRequest(req *http.Request) *Tracer {
	tr, ok := req.Context().Value(keyTracer).(*Tracer)
	if !ok {
		return nil
	}

	return tr
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	rt := t.RoundTripper
	if rt == nil {
		rt = http.DefaultTransport
	}

	tracer := TracerFromRequest(req)
	if tracer == nil {
		return rt.RoundTrip(req)
	}

	tracer.start(req)

	ext.HTTPMethod.Set(tracer.sp, req.Method)
	ext.HTTPUrl.Set(tracer.sp, tracer.opts.urlTagFunc(req.URL))
	ext.PeerAddress.Set(tracer.sp, req.URL.Host)
	tracer.opts.spanObserver(tracer.sp, req)

	if !tracer.opts.disableInjectSpanContext {
		carrier := opentracing.HTTPHeadersCarrier(req.Header)
		_ = tracer.sp.Tracer().Inject(tracer.sp.Context(), opentracing.HTTPHeaders, carrier)
	}

	resp, err := rt.RoundTrip(req)
	if err != nil {
		tracer.sp.Finish()
		return resp, err
	}

	ext.HTTPStatusCode.Set(tracer.sp, uint16(resp.StatusCode))
	ext.HTTPStatusCode.Set(tracer.parentSpan, uint16(resp.StatusCode))
	if resp.StatusCode >= http.StatusInternalServerError {
		ext.Error.Set(tracer.sp, true)
	}

	if req.Method == "HEAD" {
		tracer.sp.Finish()
	} else {
		mu := &sync.Mutex{}
		readWriteCloser, ok := resp.Body.(io.ReadWriteCloser)
		if ok {
			resp.Body = writerCloseTracker{readWriteCloser, tracer.sp, mu}
		} else {
			resp.Body = closeTracker{resp.Body, tracer.sp, mu}
		}
	}

	return resp, nil
}

func (h *Tracer) start(req *http.Request) opentracing.Span {
	if h.parentSpan == nil {
		var spanCtx opentracing.SpanContext

		parent := opentracing.SpanFromContext(req.Context())
		if parent != nil {
			spanCtx = parent.Context()
		}

		h.parentSpan = h.tr.StartSpan(h.opts.operationName, opentracing.ChildOf(spanCtx))
		ext.SpanKindRPCClient.Set(h.parentSpan)
	}

	ctx := h.parentSpan.Context()
	h.sp = h.tr.StartSpan("HTTP "+req.Method+" :"+req.URL.Path, opentracing.ChildOf(ctx), ext.SpanKindRPCClient)

	ext.Component.Set(h.sp, h.opts.componentName)
	return h.sp
}

func (h *Tracer) Finish() {
	if h.parentSpan != nil {
		h.parentSpan.Finish()
	}
}

func (h *Tracer) Span() opentracing.Span {
	return h.parentSpan
}
