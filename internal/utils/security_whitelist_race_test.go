package utils

import (
	"sync"
	"testing"
	"time"
)

// ResetSSRFWhitelistForTest runs while an earlier test's dial may still be
// validating; both must be safe under the race detector.
func TestResetSSRFWhitelistForTestIsRaceFree(t *testing.T) {
	t.Setenv("SSRF_WHITELIST", "example.com")
	ResetSSRFWhitelistForTest()

	stop := make(chan struct{})
	started := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		close(started)
		for {
			select {
			case <-stop:
				return
			default:
			}
			if !IsSSRFWhitelisted("example.com") {
				t.Error("example.com dropped out of the whitelist")
				return
			}
			if err := validateURLForSSRFForOutbound("https://example.com/"); err != nil {
				t.Errorf("outbound validation: %v", err)
				return
			}
		}
	}()
	<-started
	deadline := time.Now().Add(300 * time.Millisecond)
	for time.Now().Before(deadline) {
		ResetSSRFWhitelistForTest()
	}
	close(stop)
	wg.Wait()
}
