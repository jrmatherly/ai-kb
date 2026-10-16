package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const scrubbed500 = `{"success":false,"error":{"code":1007,"message":"Internal server error"}}`

type statusCase struct {
	name, method, path, body string
	status                   int
	want                     string
}

func runStatusCases(t *testing.T, r http.Handler, cases []statusCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)
			require.Equal(t, tc.status, w.Code, w.Body.String())
			require.JSONEq(t, tc.want, w.Body.String())
		})
	}
}
