# size

[![Run Tests](https://github.com/gin-contrib/size/actions/workflows/go.yml/badge.svg)](https://github.com/gin-contrib/size/actions/workflows/go.yml)
[![codecov](https://codecov.io/gh/gin-contrib/size/branch/master/graph/badge.svg)](https://codecov.io/gh/gin-contrib/size)
[![Go Report Card](https://goreportcard.com/badge/github.com/gin-contrib/size)](https://goreportcard.com/report/github.com/gin-contrib/size)
[![GoDoc](https://godoc.org/github.com/gin-contrib/size?status.svg)](https://godoc.org/github.com/gin-contrib/size)
[![Join the chat at https://gitter.im/gin-gonic/gin](https://badges.gitter.im/Join%20Chat.svg)](https://gitter.im/gin-gonic/gin)

Limit size of POST requests for Gin framework

## File Upload Handling

When handling file uploads, it's **crucial** to check for middleware errors first to avoid response conflicts:

### ✅ Correct Way

```go
func fileUploadHandler(ctx *gin.Context) {
  // Always check middleware errors FIRST
  if len(ctx.Errors) > 0 {
    // Size limiter has already sent HTTP 413 response
    return
  }

  // Safe to process file upload
  if file, err := ctx.FormFile("file"); err != nil {
    ctx.JSON(http.StatusBadRequest, gin.H{
      "error": "Failed to process file: " + err.Error(),
    })
  } else {
    ctx.JSON(http.StatusOK, gin.H{
      "success":  true,
      "filename": file.Filename,
      "size":   file.Size,
    })
  }
}
```

### ❌ Incorrect Way

```go
func badHandler(ctx *gin.Context) {
  // Wrong: This will cause response conflicts
  if file, err := ctx.FormFile("file"); err != nil {
    ctx.JSON(http.StatusOK, gin.H{"msg": "fail: " + err.Error()})
  } else {
    ctx.JSON(http.StatusOK, gin.H{"msg": "ok: " + file.Filename})
  }
}
```

## How It Works

When a request exceeds the size limit, the middleware will:

1. Add an error to `ctx.Errors`
2. Set `Connection: close` header
3. Return HTTP 413 (Request Entity Too Large) status
4. Send JSON response: `{"error":"request too large"}`
5. Call `ctx.Abort()` to prevent further processing

## Best Practices

- **Always check `ctx.Errors` first** in your handlers
- Use realistic size limits (consider multipart overhead)
- Handle HTTP 413 responses properly on the client side
- Test both normal and oversized file uploads

## Size Limit Examples

```go
r.Use(limits.RequestSizeLimiter(1024))      // 1KB
r.Use(limits.RequestSizeLimiter(1024 * 1024))   // 1MB
r.Use(limits.RequestSizeLimiter(10 * 1024 * 1024)) // 10MB
```

## Common Issue: File Upload Response Conflicts

If you're experiencing duplicate JSON responses like:

```json
{"error":"request too large"}{"msg":"fail: HTTP request too large"}
```

This happens when your handler tries to send a response after the middleware has already sent one. **Solution**: Always check `ctx.Errors` first!

## Client-Side Handling

```javascript
fetch("/upload", {
  method: "POST",
  body: formData,
})
  .then((response) => {
    if (response.status === 413) {
      alert("File too large, please select a smaller file");
      return;
    }
    return response.json();
  })
  .then((data) => {
    console.log("Upload successful:", data);
  });
```

## More Examples

See the `_example/` directory for complete working examples:

- `_example/main.go` - File upload best practices
