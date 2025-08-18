package limits

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestSizeLimiterOK(t *testing.T) {
	router := gin.New()
	router.Use(RequestSizeLimiter(10))
	router.POST("/test_ok", func(c *gin.Context) {
		_, _ = io.ReadAll(c.Request.Body)
		if len(c.Errors) > 0 {
			return
		}
		c.Request.Body.Close()
		c.String(http.StatusOK, "OK")
	})
	resp := performRequest("/test_ok", "big=abc", router)

	if resp.Code != http.StatusOK {
		t.Fatalf("error posting - http status %v", resp.Code)
	}
}

func TestRequestSizeLimiterOver(t *testing.T) {
	router := gin.New()
	router.Use(RequestSizeLimiter(10))
	router.POST("/test_large", func(c *gin.Context) {
		// Check for middleware errors first
		if len(c.Errors) > 0 {
			return
		}
		_, _ = io.ReadAll(c.Request.Body)
		c.Request.Body.Close()
		c.String(http.StatusOK, "OK")
	})
	resp := performRequest("/test_large", "big=abcdefghijklmnop", router)

	if resp.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("error posting - http status %v", resp.Code)
	}
}

// Test boundary case: exactly at the limit
func TestRequestSizeLimiterExactLimit(t *testing.T) {
	router := gin.New()
	router.Use(RequestSizeLimiter(10))
	router.POST("/test_exact", func(c *gin.Context) {
		_, _ = io.ReadAll(c.Request.Body)
		if len(c.Errors) > 0 {
			return
		}
		c.Request.Body.Close()
		c.String(http.StatusOK, "OK")
	})
	// "1234567890" is exactly 10 bytes
	resp := performRequest("/test_exact", "1234567890", router)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %v, got %v", http.StatusOK, resp.Code)
	}
}

// Test empty request body
func TestRequestSizeLimiterEmptyBody(t *testing.T) {
	router := gin.New()
	router.Use(RequestSizeLimiter(10))
	router.POST("/test_empty", func(c *gin.Context) {
		_, _ = io.ReadAll(c.Request.Body)
		if len(c.Errors) > 0 {
			return
		}
		c.Request.Body.Close()
		c.String(http.StatusOK, "OK")
	})
	resp := performRequest("/test_empty", "", router)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %v, got %v", http.StatusOK, resp.Code)
	}
}

// Test headers are set correctly when over limit
func TestRequestSizeLimiterHeaders(t *testing.T) {
	router := gin.New()
	router.Use(RequestSizeLimiter(5))
	router.POST("/test_headers", func(c *gin.Context) {
		// Check for middleware errors first
		if len(c.Errors) > 0 {
			return
		}
		_, _ = io.ReadAll(c.Request.Body)
		// Should not reach here due to size limit
		c.String(http.StatusOK, "OK")
	})
	resp := performRequest("/test_headers", "toolarge", router)

	if resp.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %v, got %v", http.StatusRequestEntityTooLarge, resp.Code)
	}

	// Check Connection header is set
	if resp.Header().Get("Connection") != "close" {
		t.Fatalf("expected Connection header to be 'close', got '%s'", resp.Header().Get("Connection"))
	}
}

// Test context errors are added correctly
func TestRequestSizeLimiterContextErrors(t *testing.T) {
	router := gin.New()
	router.Use(RequestSizeLimiter(5))

	var contextErrors []error
	router.POST("/test_errors", func(c *gin.Context) {
		_, _ = io.ReadAll(c.Request.Body)
		contextErrors = make([]error, len(c.Errors))
		for i, err := range c.Errors {
			contextErrors[i] = err.Err
		}
		// Note: This test needs to capture errors, so we don't return early
	})

	performRequest("/test_errors", "toolarge", router)

	if len(contextErrors) != 1 {
		t.Fatalf("expected 1 error in context, got %d", len(contextErrors))
	}

	if contextErrors[0].Error() != "HTTP request too large" {
		t.Fatalf("expected error message 'HTTP request too large', got '%s'", contextErrors[0].Error())
	}
}

// Test chunked reading (multiple small reads)
func TestRequestSizeLimiterChunkedReading(t *testing.T) {
	router := gin.New()
	router.Use(RequestSizeLimiter(10))

	router.POST("/test_chunked", func(c *gin.Context) {
		// Check for middleware errors first
		if len(c.Errors) > 0 {
			return
		}

		// Read in small chunks to test the chunked reading logic
		buf := make([]byte, 3) // Small buffer to force multiple reads
		var total []byte

		for {
			n, err := c.Request.Body.Read(buf)
			if n > 0 {
				total = append(total, buf[:n]...)
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				break
			}
		}

		c.Request.Body.Close()
		c.String(http.StatusOK, "Read %d bytes", len(total))
	})

	// Send exactly 9 bytes (under limit)
	resp := performRequest("/test_chunked", "123456789", router)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %v, got %v", http.StatusOK, resp.Code)
	}
}

// Test Close method
func TestMaxBytesReaderClose(t *testing.T) {
	router := gin.New()
	router.Use(RequestSizeLimiter(10))

	router.POST("/test_close", func(c *gin.Context) {
		// Just close without reading
		err := c.Request.Body.Close()
		if err != nil {
			c.String(http.StatusInternalServerError, "Close failed")
			return
		}
		c.String(http.StatusOK, "OK")
	})

	resp := performRequest("/test_close", "test", router)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %v, got %v", http.StatusOK, resp.Code)
	}
}

func performRequest(target, body string, router *gin.Engine) *httptest.ResponseRecorder {
	buf := bytes.NewBufferString(body)
	r := httptest.NewRequest(http.MethodPost, target, buf)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	return w
}

// TestRealisticFileUpload demonstrates real-world scenarios with proper size limits
func TestRealisticFileUpload(t *testing.T) {
	// Use a more realistic limit - 1MB for file uploads
	router := gin.New()
	router.Use(RequestSizeLimiter(1024 * 1024)) // 1MB limit

	router.POST("/upload", func(ctx *gin.Context) {
		// ALWAYS check for middleware errors first
		if len(ctx.Errors) > 0 {
			// Middleware has already handled the error response
			return
		}

		// Safe to proceed with file processing
		file, err := ctx.FormFile("file")
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Failed to process file: " + err.Error(),
			})
			return
		}

		ctx.JSON(http.StatusOK, gin.H{
			"success":  true,
			"message":  "File uploaded successfully",
			"filename": file.Filename,
			"size":     file.Size,
		})
	})

	t.Run("NormalFileUpload", func(t *testing.T) {
		// Test with normal file (should succeed)
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		part, _ := writer.CreateFormFile("file", "test.txt")
		part.Write([]byte("This is a normal file content"))
		writer.Close()

		req := httptest.NewRequest("POST", "/upload", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
		}
		t.Logf("Normal file response: %s", w.Body.String())
	})

	t.Run("VeryLargeFileUpload", func(t *testing.T) {
		// Test with very large file (should be rejected by middleware)
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		part, _ := writer.CreateFormFile("file", "huge_file.bin")

		// Write 2MB of data - exceeds 1MB limit
		largeContent := bytes.Repeat([]byte("X"), 2*1024*1024)
		part.Write(largeContent)
		writer.Close()

		req := httptest.NewRequest("POST", "/upload", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Should get 413 from middleware
		if w.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("Expected status %d, got %d", http.StatusRequestEntityTooLarge, w.Code)
		}

		t.Logf("Large file response code: %d", w.Code)
		t.Logf("Large file response body: %s", w.Body.String())

		// Should only contain middleware response
		expectedStart := `{"error":"request too large"}`
		actualBody := w.Body.String()
		if !bytes.HasPrefix([]byte(actualBody), []byte(expectedStart)) {
			t.Errorf("Expected response to start with %s, got: %s", expectedStart, actualBody)
		}

		// Check Connection header
		if w.Header().Get("Connection") != "close" {
			t.Errorf("Expected Connection header to be 'close', got '%s'", w.Header().Get("Connection"))
		}
	})
}
