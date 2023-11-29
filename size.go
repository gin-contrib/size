package limits

import (
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

var ErrTooLarge = fmt.Errorf("HTTP request too large")

type TooLargeCallback func(ctx *gin.Context)
type opt func(*maxBytesReader) error

type maxBytesReader struct {
	ctx        *gin.Context
	rdr        io.ReadCloser
	remaining  int64
	wasAborted bool
	sawEOF     bool
	callback   TooLargeCallback
}

func (mbr *maxBytesReader) tooLarge() (n int, err error) {
	n, err = 0, ErrTooLarge

	if !mbr.wasAborted {
		mbr.wasAborted = true
		ctx := mbr.ctx
		_ = ctx.Error(err)

		if mbr.callback != nil {
			mbr.callback(ctx)
		}

		ctx.Header("connection", "close")
		ctx.String(http.StatusRequestEntityTooLarge, "request too large")
		ctx.AbortWithStatus(http.StatusRequestEntityTooLarge)
	}
	return
}

func (mbr *maxBytesReader) Read(p []byte) (n int, err error) {
	toRead := mbr.remaining
	if mbr.remaining == 0 {
		if mbr.sawEOF {
			return mbr.tooLarge()
		}
		// The underlying io.Reader may not return (0, io.EOF)
		// at EOF if the requested size is 0, so read 1 byte
		// instead. The io.Reader docs are a bit ambiguous
		// about the return value of Read when 0 bytes are
		// requested, and {bytes,strings}.Reader gets it wrong
		// too (it returns (0, nil) even at EOF).
		toRead = 1
	}
	if int64(len(p)) > toRead {
		p = p[:toRead]
	}
	n, err = mbr.rdr.Read(p)
	if err == io.EOF {
		mbr.sawEOF = true
	}
	if mbr.remaining == 0 {
		// If we had zero bytes to read remaining (but hadn't seen EOF)
		// and we get a byte here, that means we went over our limit.
		if n > 0 {
			return mbr.tooLarge()
		}
		return 0, err
	}
	mbr.remaining -= int64(n)
	if mbr.remaining < 0 {
		mbr.remaining = 0
	}
	return n, err
}

func (mbr *maxBytesReader) Close() error {
	return mbr.rdr.Close()
}

func WithCallback(cb TooLargeCallback) opt {
	return func(mbr *maxBytesReader) error {
		if cb == nil {
			return fmt.Errorf("nil callback")
		}
		mbr.callback = cb
		return nil
	}
}

// RequestSizeLimiter returns a middleware that limits the size of request
// When a request is over the limit, the following will happen:
// * Error will be added to the context
// * Connection: close header will be set
// * Error 413 will be sent to the client (http.StatusRequestEntityTooLarge)
// * Current context will be aborted
func RequestSizeLimiter(limit int64, opts ...opt) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		mbr := &maxBytesReader{
			ctx:        ctx,
			rdr:        ctx.Request.Body,
			remaining:  limit,
			wasAborted: false,
			sawEOF:     false,
		}

		for _, opt := range opts {
			opt(mbr)
		}

		ctx.Request.Body = mbr
		ctx.Next()
	}
}
