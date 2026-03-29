package openai

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/interfaces"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/api/handlers"
	sdkconfig "github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
)

func TestForwardResponsesStreamSeparatesDataOnlySSEChunks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	base := handlers.NewBaseAPIHandlers(&sdkconfig.SDKConfig{}, nil)
	h := NewOpenAIResponsesAPIHandler(base)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		t.Fatalf("expected gin writer to implement http.Flusher")
	}

	data := make(chan []byte, 2)
	errs := make(chan *interfaces.ErrorMessage)
	data <- []byte("data: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"function_call\",\"arguments\":\"{}\"}}")
	data <- []byte("data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp-1\",\"output\":[]}}")
	close(data)
	close(errs)

	h.forwardResponsesStream(c, flusher, func(error) {}, data, errs)
	body := recorder.Body.String()

	if !strings.Contains(body, "data: {\"type\":\"response.output_item.done\"") {
		t.Fatalf("expected first SSE data chunk, got: %q", body)
	}
	if !strings.Contains(body, "\n\ndata: {\"type\":\"response.completed\"") {
		t.Fatalf("expected blank-line separation before second SSE event, got: %q", body)
	}
	if strings.Contains(body, "arguments\":\"{}\"}}data: {\"type\":\"response.completed\"") {
		t.Fatalf("second SSE event was concatenated onto first event body: %q", body)
	}
}
