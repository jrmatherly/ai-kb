package client

import "testing"

func TestNewAPIErrorReadsServerCode(t *testing.T) {
	for body, want := range map[string]int{
		`{"success":false,"error":{"code":1003,"message":"not found"}}`: ServerErrNotFound,
		`{"code":1010,"message":"bad"}`:                                 ServerErrValidation,
		`{"success":false,"error":"plain message"}`:                     0,
		`not json`: 0,
	} {
		if got := newAPIError(404, []byte(body)).Code; got != want {
			t.Errorf("%s: code %d, want %d", body, got, want)
		}
	}
}
