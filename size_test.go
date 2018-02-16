package limits

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestSizeLimiterOK(t *testing.T) {
	router := gin.New()
	router.Use(RequestSizeLimiter(10))
	router.POST("/test_ok", func(c *gin.Context) {
		ioutil.ReadAll(c.Request.Body)
		if len(c.Errors) > 0 {
			return
		}
		c.Request.Body.Close()
		c.String(http.StatusOK, "OK")
	})
	resp := performRequest(http.MethodPost, "/test_ok", "big=abc", router)

	if resp.Code != http.StatusOK {
		t.Fatalf("error posting - http status %v", resp.Code)
	}
}

func TestRequestSizeLimiterOver(t *testing.T) {
	router := gin.New()
	router.Use(RequestSizeLimiter(10))
	router.POST("/test_large", func(c *gin.Context) {
		ioutil.ReadAll(c.Request.Body)
		if len(c.Errors) > 0 {
			return
		}
		c.Request.Body.Close()
		c.String(http.StatusOK, "OK")
	})
	resp := performRequest(http.MethodPost, "/test_large", "big=abcdefghijklmnop", router)

	if resp.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("error posting - http status %v", resp.Code)
	}
}

func performRequest(method, target, body string, router *gin.Engine) *httptest.ResponseRecorder {
	var buf *bytes.Buffer
	if body != "" {
		buf = new(bytes.Buffer)
		buf.WriteString(body)
	}
	r := httptest.NewRequest(method, target, buf)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	return w
}
