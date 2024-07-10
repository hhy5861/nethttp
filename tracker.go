package nethttp

import (
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"
	"io"
	"sync"
)

type (
	closeTracker struct {
		io.ReadCloser
		sp opentracing.Span
		*sync.Mutex
	}

	writerCloseTracker struct {
		io.ReadWriteCloser
		sp opentracing.Span
		*sync.Mutex
	}
)

func (c closeTracker) Close() error {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()

	err := c.ReadCloser.Close()
	c.sp.LogFields(log.String("event", "ClosedBody"))
	c.sp.Finish()

	return err
}

func (c writerCloseTracker) Close() error {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()

	err := c.ReadWriteCloser.Close()
	c.sp.LogFields(log.String("event", "ClosedBody"))
	c.sp.Finish()

	return err
}
