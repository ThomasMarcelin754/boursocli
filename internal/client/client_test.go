package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func mkJWT(claims map[string]any) string {
	hdr := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	pl, _ := json.Marshal(claims) //nolint:errchkjson // test helper, map[string]any literals never fail
	return hdr + "." + base64.RawURLEncoding.EncodeToString(pl) + ".sig"
}

func TestBearerFromCookie(t *testing.T) {
	good := mkJWT(map[string]any{"exp": float64(time.Now().Add(time.Hour).Unix()), "userHash": "H123"})
	c := New("a=1; brsxds_deadbeefdeadbeefdeadbeefdeadbeef="+good+"; b=2", "")
	if jwt, uh, ok := c.bearerFromCookie(); !ok || uh != "H123" || jwt != good {
		t.Fatalf("valid brsxds JWT not accepted: ok=%v uh=%q", ok, uh)
	}
	// expired → declined
	exp := mkJWT(map[string]any{"exp": float64(time.Now().Add(-time.Hour).Unix()), "userHash": "H"})
	if _, _, ok := New("brsxds_x="+exp, "").bearerFromCookie(); ok {
		t.Fatal("expired brsxds JWT must be declined")
	}
	// no userHash claim → declined
	nouh := mkJWT(map[string]any{"exp": float64(time.Now().Add(time.Hour).Unix())})
	if _, _, ok := New("brsxds_x="+nouh, "").bearerFromCookie(); ok {
		t.Fatal("brsxds JWT without userHash must be declined")
	}
	// absent / malformed → declined (no regression to existing error path)
	if _, _, ok := New("sid=1; other=2", "").bearerFromCookie(); ok {
		t.Fatal("no brsxds cookie must yield ok=false")
	}
	if _, _, ok := New("brsxds_x=notajwt", "").bearerFromCookie(); ok {
		t.Fatal("malformed brsxds value must yield ok=false")
	}
}

func TestPredicates(t *testing.T) {
	if !isTrustedHost("clients.boursobank.com") || !isTrustedHost("api.boursorama.com") ||
		!isTrustedHost("boursobank.com") {
		t.Fatal("trusted hosts misclassified")
	}
	if isTrustedHost("evil.com") || isTrustedHost("boursobank.com.evil.com") {
		t.Fatal("hostile host accepted")
	}
	if !isThrottled(503, nil) || !isThrottled(401, []byte("401 V Not Authorized")) {
		t.Fatal("throttle not detected")
	}
	if isThrottled(200, []byte("ok")) || isThrottled(401, []byte(`{"code":401}`)) {
		t.Fatal("false throttle")
	}
	if !isBankSessionExpired(401, []byte(`{"code":10006,"message":"x"}`)) ||
		!isBankSessionExpired(401, []byte(`{"code":401,"message":"JWT Token not found"}`)) {
		t.Fatal("bank expiry not detected")
	}
	if isBankSessionExpired(200, []byte(`{"ok":true}`)) {
		t.Fatal("false bank expiry")
	}
	if d := backoff(0); d <= 0 {
		t.Fatal("backoff non-positive")
	}
}

func TestDoHeaders(t *testing.T) {
	var gotAuth, gotCookie, gotXRW string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("authorization")
		gotCookie = r.Header.Get("cookie")
		gotXRW = r.Header.Get("x-requested-with")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	c := New("ck=1", "")
	if c.ua != defaultUA {
		t.Fatal("default UA not applied")
	}
	c.Bearer = "JWT"

	_, st, _, err := c.do(context.Background(), http.MethodGet, srv.URL, "application/json", true)
	if err != nil || st != 200 {
		t.Fatalf("bearer do: %v st=%d", err, st)
	}
	if gotAuth != "Bearer JWT" || gotCookie != "" || gotXRW != "" {
		t.Fatalf("bearer headers wrong: auth=%q cookie=%q", gotAuth, gotCookie)
	}
	_, _, _, _ = c.do(context.Background(), http.MethodGet, srv.URL, "text/html", false)
	if gotCookie != "ck=1" || gotXRW != "XMLHttpRequest" {
		t.Fatalf("cookie headers wrong: cookie=%q xrw=%q", gotCookie, gotXRW)
	}
}

func TestResilientThrottleThenSuccess(t *testing.T) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&n, 1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable) // throttle once
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	c := New("ck=1", "")
	// backoff(0) ~2s; keep the test fast by shrinking via a tiny ctx is not
	// possible — instead assert it recovers (2s is acceptable for one case).
	b, st, err := c.resilientGet(context.Background(), srv.URL, "application/json", false)
	if err != nil || st != 200 || !strings.Contains(string(b), "ok") {
		t.Fatalf("did not recover from throttle: st=%d err=%v", st, err)
	}
	if atomic.LoadInt32(&n) < 2 {
		t.Fatal("did not retry after throttle")
	}
}

func TestResilientBankExpiryRefreshRetry(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "session/auth/refresh") {
			w.WriteHeader(http.StatusOK)
			return
		}
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"code":10006,"message":"Votre session a expirée"}`))
			return
		}
		_, _ = w.Write([]byte(`{"recovered":true}`))
	}))
	defer srv.Close()
	apiBase = srv.URL // seam: Refresh() targets apiBase
	defer func() { apiBase = "https://api.boursobank.com/services/api/v1.7" }()
	c := New("ck=1", "")
	c.Bearer = "JWT"
	b, st, err := c.resilientGet(context.Background(), srv.URL, "application/json", true)
	if err != nil || st != 200 || !strings.Contains(string(b), "recovered") {
		t.Fatalf("refresh+retry failed: st=%d err=%v body=%s", st, err, b)
	}
}

func TestRefreshAndBootstrap(t *testing.T) {
	html := `garbage "USER_HASH":"abc123" more "DEFAULT_API_BEARER":"JWT.tok" end`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "refresh") {
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()
	apiBase, dashboard = srv.URL, srv.URL+"/"
	defer func() {
		apiBase = "https://api.boursobank.com/services/api/v1.7"
		dashboard = "https://clients.boursobank.com/"
	}()
	c := New("ck=1", "")
	if err := c.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if err := c.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	if c.Bearer != "JWT.tok" || c.UserHash != "abc123" {
		t.Fatalf("scrape wrong: bearer=%q hash=%q", c.Bearer, c.UserHash)
	}
}

func TestBootstrapDeadSession(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html>connexion</html>`)) // no bearer blob
	}))
	defer srv.Close()
	dashboard = srv.URL + "/"
	defer func() { dashboard = "https://clients.boursobank.com/" }()
	c := New("ck=1", "")
	if err := c.Bootstrap(context.Background()); err == nil {
		t.Fatal("expected loud error on dead session")
	}
}

func TestRedirectCookieAllowlist(t *testing.T) {
	// untrusted redirect target must NOT receive the cookie
	var leaked string
	evil := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaked = r.Header.Get("cookie")
		_, _ = w.Write([]byte("x"))
	}))
	defer evil.Close()
	src := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, evil.URL, http.StatusFound)
	}))
	defer src.Close()
	c := New("secret=1", "")
	_, _, _, _ = c.do(context.Background(), http.MethodGet, src.URL, "text/html", false)
	if leaked != "" {
		t.Fatalf("session cookie leaked to untrusted redirect host: %q", leaked)
	}
}
