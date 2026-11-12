package middleware

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestLoggerWritesMethodPathStatusAndDuration(t *testing.T) {
	var buf bytes.Buffer

	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(Logger())
	router.GET("/hello", func(ctx *gin.Context) { ctx.Status(http.StatusTeapot) })

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/hello?x=1", nil))

	line := buf.String()
	if !strings.Contains(line, "GET /hello 418 ") {
		t.Errorf("log line %q, want method, path and status", line)
	}

	// The last field is a duration such as "12.3µs".
	if !strings.HasSuffix(strings.TrimSpace(line), "s") {
		t.Errorf("log line %q does not end with a duration", line)
	}
}
