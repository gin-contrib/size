package main

import (
	"net/http"

	limits "github.com/gin-contrib/size"
	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()

	// Use a realistic limit for file uploads (e.g., 10MB)
	r.Use(limits.RequestSizeLimiter(10 * 1024 * 1024))

	r.POST("/", fileUploadHandler)
	_ = r.Run(":8080")
}

// fileUploadHandler demonstrates the BEST PRACTICE for handling file uploads with size limits
func fileUploadHandler(ctx *gin.Context) {
	// CRITICAL: Always check for middleware errors FIRST
	// This prevents conflicts with middleware responses
	if len(ctx.Errors) > 0 {
		// The size limiter middleware has already:
		// 1. Set HTTP status 413 (Request Entity Too Large)
		// 2. Sent JSON response: {"error": "request too large"}
		// 3. Set Connection: close header
		// 4. Called ctx.Abort() to prevent further processing
		//
		// DO NOT try to send another response - just return
		return
	}

	// Now it's safe to process the file upload
	file, err := ctx.FormFile("file")
	if err != nil {
		// Handle other types of errors (not size-related)
		ctx.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Failed to process file upload: " + err.Error(),
			"code":    "FORM_ERROR",
		})
		return
	}

	// Successful upload
	ctx.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  "File uploaded successfully",
		"filename": file.Filename,
		"size":     file.Size,
		"code":     "UPLOAD_SUCCESS",
	})
}
