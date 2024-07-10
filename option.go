package nethttp

import (
	"github.com/opentracing/opentracing-go"
	"net/http"
	"net/url"
)

type (
	clientOptions struct {
		operationName            string
		componentName            string
		urlTagFunc               func(u *url.URL) string
		disableClientTrace       bool
		disableInjectSpanContext bool
		spanObserver             func(span opentracing.Span, r *http.Request)
	}



	ClientOption func(*clientOptions)
)

func OperationName(operationName string) ClientOption {
	return func(options *clientOptions) {
		options.operationName = operationName
	}
}

func URLTagFunc(f func(u *url.URL) string) ClientOption {
	return func(options *clientOptions) {
		options.urlTagFunc = f
	}
}

func ComponentName(componentName string) ClientOption {
	return func(options *clientOptions) {
		options.componentName = componentName
	}
}

func ClientTrace(enabled bool) ClientOption {
	return func(options *clientOptions) {
		options.disableClientTrace = !enabled
	}
}

func InjectSpanContext(enabled bool) ClientOption {
	return func(options *clientOptions) {
		options.disableInjectSpanContext = !enabled
	}
}

func ClientSpanObserver(f func(span opentracing.Span, r *http.Request)) ClientOption {
	return func(options *clientOptions) {
		options.spanObserver = f
	}
}
