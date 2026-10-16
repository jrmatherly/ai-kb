package cmdutil

import (
	"errors"
	"fmt"
	"testing"

	sdk "github.com/jrmatherly/ai-kb/client"
	"github.com/stretchr/testify/assert"
)

func TestClassifyHTTPError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want ErrorCode
	}{
		{"nil", nil, ""},
		{"non-HTTP transport", errors.New("dial tcp: lookup host: no such host"), CodeNetworkError},
		{"401", fmt.Errorf("HTTP error 401: invalid token"), CodeAuthUnauthenticated},
		{"403", fmt.Errorf("HTTP error 403: forbidden"), CodeAuthForbidden},
		{"404", fmt.Errorf("HTTP error 404: kb not found"), CodeResourceNotFound},
		{"409", fmt.Errorf("HTTP error 409: already exists"), CodeResourceAlreadyExists},
		{"429", fmt.Errorf("HTTP error 429: slow down"), CodeServerRateLimited},
		{"500", fmt.Errorf("HTTP error 500: internal"), CodeServerError},
		{"503", fmt.Errorf("HTTP error 503: unavailable"), CodeServerError},
		{"400 generic", fmt.Errorf("HTTP error 400: bad input"), CodeInputInvalidArgument},
		{"422 unprocessable", fmt.Errorf("HTTP error 422: validation"), CodeInputInvalidArgument},
		{"malformed status", fmt.Errorf("HTTP error abc: oops"), CodeServerError},
		{"missing colon", fmt.Errorf("HTTP error 404 no colon"), CodeServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, ClassifyHTTPError(tc.err))
		})
	}
}

func TestClassifySDKError_SSEStreamTerminal(t *testing.T) {
	err := sdk.NewSSEStreamError("boom")
	assert.Equal(t, CodeServerError, ClassifySDKError(err))
	assert.Equal(t, CodeNetworkError, ClassifyHTTPError(err))
}

func TestClassifyHTTPError_500Generic_StaysServerError(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{
			name: "generic 500",
			body: "HTTP error 500: internal server error",
		},
		{
			name: "config file not found in stack trace",
			body: "HTTP error 500: panic: config file not found in /etc/app/config.yaml",
		},
		{
			name: "not found substring without server code",
			body: `HTTP error 500: {"message":"something not found","other":true}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := fmt.Errorf("%s", tc.body)
			if got := ClassifyHTTPError(err); got != CodeServerError {
				t.Errorf("expected server.error for generic 500; got %v", got)
			}
		})
	}
}
