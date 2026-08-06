package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAccountHandlerTestImageRequiresExplicitConfirmation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/7/test-image", strings.NewReader(`{"confirm":false}`))
	request.Header.Set("Content-Type", "application/json")
	context, _ := gin.CreateTestContext(recorder)
	context.Request = request
	context.Params = gin.Params{{Key: "id", Value: "7"}}

	(&AccountHandler{}).TestImage(context)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "api_key")
}
