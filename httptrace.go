package nethttp

import (
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/opentracing/opentracing-go/log"
	"net/http/httptrace"
)

type (
	Tracer struct {
		tr         opentracing.Tracer
		parentSpan opentracing.Span
		sp         opentracing.Span
		opts       *clientOptions
	}
)

func (h *Tracer) clientTrace() *httptrace.ClientTrace {
	return &httptrace.ClientTrace{
		GetConn:              h.getConn,
		GotConn:              h.gotConn,
		PutIdleConn:          h.putIdleConn,
		GotFirstResponseByte: h.gotFirstResponseByte,
		Got100Continue:       h.got100Continue,
		DNSStart:             h.dnsStart,
		DNSDone:              h.dnsDone,
		ConnectStart:         h.connectStart,
		ConnectDone:          h.connectDone,
		WroteHeaders:         h.wroteHeaders,
		Wait100Continue:      h.wait100Continue,
		WroteRequest:         h.wroteRequest,
	}
}

func (h *Tracer) getConn(hostPort string) {
	h.sp.LogFields(log.String("event", "GetConn"), log.String("hostPort", hostPort))
}

func (h *Tracer) gotConn(info httptrace.GotConnInfo) {
	h.sp.SetTag("net/http.reused", info.Reused)
	h.sp.SetTag("net/http.was_idle", info.WasIdle)
	h.sp.LogFields(log.String("event", "GotConn"))
}

func (h *Tracer) putIdleConn(error) {
	h.sp.LogFields(log.String("event", "PutIdleConn"))
}

func (h *Tracer) gotFirstResponseByte() {
	h.sp.LogFields(log.String("event", "GotFirstResponseByte"))
}

func (h *Tracer) got100Continue() {
	h.sp.LogFields(log.String("event", "Got100Continue"))
}

func (h *Tracer) dnsStart(info httptrace.DNSStartInfo) {
	h.sp.LogFields(
		log.String("event", "DNSStart"),
		log.String("host", info.Host),
	)
}

func (h *Tracer) dnsDone(info httptrace.DNSDoneInfo) {
	fields := []log.Field{log.String("event", "DNSDone")}
	for _, addr := range info.Addrs {
		fields = append(fields, log.String("addr", addr.String()))
	}

	if info.Err != nil {
		fields = append(fields, log.Error(info.Err))
	}

	h.sp.LogFields(fields...)
}

func (h *Tracer) connectStart(network, addr string) {
	h.sp.LogFields(
		log.String("event", "ConnectStart"),
		log.String("network", network),
		log.String("addr", addr),
	)
}

func (h *Tracer) connectDone(network, addr string, err error) {
	if err != nil {
		h.sp.LogFields(
			log.String("message", "ConnectDone"),
			log.String("network", network),
			log.String("addr", addr),
			log.String("event", "error"),
			log.Error(err),
		)
	} else {
		h.sp.LogFields(
			log.String("event", "ConnectDone"),
			log.String("network", network),
			log.String("addr", addr),
		)
	}
}

func (h *Tracer) wroteHeaders() {
	h.sp.LogFields(log.String("event", "WroteHeaders"))
}

func (h *Tracer) wait100Continue() {
	h.sp.LogFields(log.String("event", "Wait100Continue"))
}

func (h *Tracer) wroteRequest(info httptrace.WroteRequestInfo) {
	if info.Err != nil {
		h.sp.LogFields(
			log.String("message", "WroteRequest"),
			log.String("event", "error"),
			log.Error(info.Err),
		)

		ext.Error.Set(h.sp, true)
	} else {
		h.sp.LogFields(log.String("event", "WroteRequest"))
	}
}
