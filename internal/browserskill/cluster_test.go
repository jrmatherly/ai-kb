package browserskill

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestReconnectAfterServerRestartKeepsAuthorizationAndPausesTasks(t *testing.T) {
	binary := os.Getenv("BROWSERSKILL_TEST_BINARY")
	if binary == "" {
		t.Skip("native daemon required")
	}
	store := testStore(t)
	m := NewManager(store)
	m.binary = binary
	var current atomic.Pointer[Manager]
	current.Store(m)
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { current.Load().ServeHTTP(w, r) }),
	)
	t.Cleanup(server.Close)
	t.Cleanup(func() { current.Load().Close() })
	m.publicURL = "ws" + strings.TrimPrefix(server.URL, "http") + "/extension"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	scope := Scope{1, "alice"}
	link, err := m.Pair(ctx, scope, "")
	require.NoError(t, err)
	link = redeemTestPair(ctx, t, m, link)
	connectSharedFixture(ctx, t, m, scope, link, "chrome")
	require.NoError(t, m.Control(ctx, scope, "chat", "start"))
	oldID := m.Status(scope, "chat").SessionID
	m.Close()
	next := NewManager(store)
	next.binary = binary
	next.publicURL = m.publicURL
	current.Store(next)
	connectSharedFixture(ctx, t, next, scope, link, "chrome")
	status, err := next.GetStatus(ctx, scope, "chat")
	require.NoError(t, err)
	require.True(t, status.Connected)
	require.True(t, status.Paused)
	require.True(t, status.Selected)
	require.Empty(t, status.SessionID)
	require.NoError(t, next.Control(ctx, scope, "chat", "select"))
	_, err = next.Call(ctx, scope, "chat", "snapshot", nil)
	require.Error(t, err)
	require.NoError(t, next.Control(ctx, scope, "chat", "resume"))
	require.NotEqual(t, oldID, next.Status(scope, "chat").SessionID)
	_, err = next.Call(ctx, scope, "chat", "snapshot", nil)
	require.NoError(t, err)
	require.NoError(t, next.Control(ctx, scope, "chat", "stop"))
	rows, err := store.tasks(ctx, scope)
	require.NoError(t, err)
	require.Empty(t, rows)
}

func TestRequestsRouteToBrowserOwnerAcrossReplicas(t *testing.T) {
	owner, ctx := sharedTestManager(t)
	owner.clusterSecret = strings.Repeat("cluster-secret-", 3)
	server := httptest.NewServer(http.HandlerFunc(owner.InternalHTTP))
	defer server.Close()
	owner.internalURL = server.URL
	follower := NewManager(owner.store)
	follower.binary = owner.binary
	follower.publicURL = owner.publicURL
	follower.clusterSecret = owner.clusterSecret
	t.Cleanup(follower.Close)
	scope := Scope{1, "alice"}
	fixture := connectSharedFixture(ctx, t, owner, scope, "", "chrome")
	require.NoError(t, follower.Control(ctx, scope, "chat", "start"))
	require.Nil(t, follower.daemon, "routing must not launch a second daemon")
	status, err := follower.GetStatus(ctx, scope, "chat")
	require.NoError(t, err)
	require.True(t, status.Connected)
	require.Equal(t, "0.3.0", status.ExtensionVersion)
	account, err := follower.Account(ctx, scope)
	require.NoError(t, err)
	require.Equal(t, "0.3.0", account.ExtensionVersion, "account version follows the remote connection owner")
	_, err = follower.Call(ctx, scope, "chat", "navigate", map[string]any{"url": "https://example.com"})
	require.NoError(t, err)
	require.Equal(t, status.SessionID, (<-fixture.calls)["session_id"])
	_, err = follower.Call(ctx, scope, "chat", "click", map[string]any{"ref": "e99"})
	var rpcErr *RPCError
	require.ErrorAs(t, err, &rpcErr)
	require.EqualError(t, err, "not_found: fixture ref missing")
	require.JSONEq(t, `{"reason":"ref_not_found","effect_state":"none","tab_id":1}`, string(rpcErr.Data))
	require.NoError(t, follower.Control(ctx, scope, "chat", "pause"))
	require.True(t, owner.Status(scope, "chat").Paused)
	require.NoError(t, follower.Focus(ctx, scope, "chat"))
	require.True(t, owner.Status(scope, "chat").Paused, "locating a window must not resume automation")
	require.NoError(t, follower.FinishTurn(ctx, scope, "chat", true))
	require.True(t, owner.Status(scope, "chat").Idle)
	require.NoError(t, follower.Revoke(ctx, scope))
	_, err = owner.Call(ctx, scope, "chat", "snapshot", nil)
	require.ErrorIs(t, err, ErrAuthorization)
}

func TestInternalRPCRejectsForgeryAndReplay(t *testing.T) {
	m := NewManager(testStore(t))
	m.clusterSecret = strings.Repeat("test-key", 8)
	request := clusterRequest{Node: m.nodeID, Scope: Scope{1, "alice"}, Session: "chat", Operation: "status"}
	body, err := json.Marshal(request)
	require.NoError(t, err)
	timestamp, nonce := strconv.FormatInt(time.Now().Unix(), 10), randomID()
	call := func(sig string) int {
		r := httptest.NewRequest(http.MethodPost, internalPath, bytes.NewReader(body))
		r.Header.Set("X-Browser-Timestamp", timestamp)
		r.Header.Set("X-Browser-Nonce", nonce)
		r.Header.Set("X-Browser-Signature", sig)
		w := httptest.NewRecorder()
		m.InternalHTTP(w, r)
		return w.Code
	}
	require.Equal(t, 401, call("forged"))
	sig := signRPC(m.clusterSecret, timestamp+"\n"+nonce, body)
	require.Equal(t, 200, call(sig))
	require.Equal(t, 409, call(sig))
}

// The owning replica names the failure kind so the follower maps it to the same
// sentinel a local failure produces; a pre-kind owner is matched on the store
// message, and a transport failure is an upstream error.
func TestRouteMapsOwnerErrorKindsToSentinels(t *testing.T) {
	store := testStore(t)
	scope := Scope{1, "alice"}
	var mu sync.Mutex
	reply := clusterResponse{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(reply)
	}))
	defer server.Close()
	now := time.Now()
	require.NoError(t, store.db.Create(&DeviceRecord{
		ScopeKey: scope.key(), ID: "device-1", Tenant: 1, User: "alice", TokenHash: "hash-1",
		ExpiresAt: now.Add(time.Hour), Owner: "other-node", LeaseKey: "lease", OwnerURL: server.URL,
		LeaseUntil: now.Add(time.Hour),
	}).Error)
	m := NewManager(store)
	t.Cleanup(m.Close)
	m.clusterSecret = strings.Repeat("cluster-secret-", 3)
	route := func() error {
		_, forwarded, err := m.route(context.Background(), scope, "chat", "call", "navigate", nil)
		require.True(t, forwarded)
		return err
	}
	for _, tc := range []struct {
		name  string
		reply clusterResponse
		want  error
	}{
		{"store kind", clusterResponse{Error: "owner store text", Kind: "store"}, ErrStore},
		{"authorization kind", clusterResponse{Error: "owner auth text", Kind: "authorization"}, ErrAuthorization},
		{
			"upstream kind",
			clusterResponse{Error: ErrUpstream.Error() + ": BrowserSkill daemon unavailable", Kind: "upstream"},
			ErrUpstream,
		},
		{"pre-kind owner store message", clusterResponse{Error: ErrStore.Error()}, ErrStore},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mu.Lock()
			reply = tc.reply
			mu.Unlock()
			err := route()
			require.ErrorIs(t, err, tc.want)
			if tc.reply.Kind == "upstream" {
				require.EqualError(t, err, "browser upstream failed: BrowserSkill daemon unavailable",
					"the prefix is not doubled")
			}
		})
	}
	server.Close()
	require.ErrorIs(t, route(), ErrUpstream, "a transport failure is an upstream error")
}
