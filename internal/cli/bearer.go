package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/thomasmarcelin754/boursocli/internal/client"
	"github.com/thomasmarcelin754/boursocli/internal/out"
)

// apiErr is BoursoBank's JSON error envelope ({"code":N,"message":"…"}).
type apiErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func parseAPIErr(body []byte) (apiErr, bool) {
	var e apiErr
	if json.Unmarshal(body, &e) == nil && e.Code != 0 && e.Message != "" {
		return e, true
	}
	return apiErr{}, false
}

// notHeldCodes = the product/tier is simply not owned by this customer.
// These are NOT auth/integration failures — surface as
// "unavailable" and exit 0, never as a silent empty nor a hard error.
var notHeldCodes = map[int]string{
	10100: "product not held by this customer",
	10013: "feature/tier not held by this customer",
}

// getJSON does a Bearer GET with strict, loud handling:
//   - transport error  → error
//   - non-200          → error (with body snippet) UNLESS it is a not-held code
//   - not valid JSON   → error
//   - not-held code    → ok=false, unavailable=true, exit 0 (handled, printed)
//
// Returns (body, handled). When handled==true the caller must return nil
// (output already emitted); otherwise body is a valid 200 JSON payload.
func getJSON(ctx context.Context, cl *client.Client, resource string) ([]byte, bool, error) {
	body, status, err := cl.API(ctx, resource)
	if err != nil {
		return nil, false, err
	}
	// Success path FIRST: a 200 with valid JSON is the payload — never
	// re-interpret it as an error envelope (a legit body may carry
	// code/message fields of its own).
	if status == 200 && json.Valid(body) {
		return body, false, nil
	}
	// Non-200 (or 200 non-JSON): classify per the error taxonomy.
	if e, ok := parseAPIErr(body); ok {
		if reason, held := notHeldCodes[e.Code]; held {
			_ = out.Data(map[string]any{
				"ok": false, "unavailable": true,
				"code": e.Code, "message": e.Message, "reason": reason,
				"resource": resource,
			})
			return nil, true, nil
		}
		if e.Code == 10006 && strings.HasPrefix(resource, "trading/") {
			return nil, false, fmt.Errorf("%s → 10006 %q : la session n’est PAS élevée bourse (ce n’est PAS un mur permanent). Ouvrir l’espace Bourse/ORD dans le Chrome connecté pour élever le jar .boursorama.com, puis réessayer", resource, e.Message)
		}
		return nil, false, fmt.Errorf("%s → erreur API code %d : %s (HTTP %d)", resource, e.Code, e.Message, status)
	}
	if status != 200 {
		return nil, false, fmt.Errorf("%s → HTTP %d : %s", resource, status, snippet(body))
	}
	return nil, false, fmt.Errorf("%s → HTTP 200 mais le corps n’est pas du JSON valide : %s", resource, snippet(body))
}

// rawStr renders a JSON value of unspecified type (amount's type is not
// pinned) as a string for the lossy table view, without inventing
// a numeric shape: a JSON string loses its quotes, anything else is verbatim.
func rawStr(r json.RawMessage) string {
	s := string(r)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		var u string
		if json.Unmarshal(r, &u) == nil {
			return u
		}
	}
	return s
}

// decodeRows extracts an array under key from a JSON object body as untyped
// rows ([]map[string]json.RawMessage). Used by the lossy --format table view
// so a field being an object/array/number/string never type-crashes the table
// (the exhaustive, type-faithful output is the default JSON via out.Raw).
func decodeRows(body []byte, key string) ([]map[string]json.RawMessage, error) {
	var env map[string]json.RawMessage
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("décodage de l’enveloppe %s pour le tableau : %w", key, err)
	}
	raw, ok := env[key]
	if !ok {
		return nil, fmt.Errorf("tableau : pas de tableau %q dans la réponse (clés : %v)", key, keysOf(env))
	}
	var rows []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("décodage de %s[] pour le tableau : %w", key, err)
	}
	return rows, nil
}

func keysOf(m map[string]json.RawMessage) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

// addAccountFlag binds a reusable --account flag and returns the bound pointer.
func addAccountFlag(c *cobra.Command) *string {
	var s string
	c.Flags().StringVar(&s, "account", "", "accountKey (32-hex) ou type : cav|ord|card")
	return &s
}

// resolvePicked = resolveAccounts + pickAccount, the common preamble.
func resolvePicked(ctx context.Context, sel string) (*client.Client, acct, error) {
	cl, accs, _, err := resolveAccounts(ctx)
	if err != nil {
		return nil, acct{}, err
	}
	a, err := pickAccount(accs, sel)
	if err != nil {
		return nil, acct{}, err
	}
	return cl, a, nil
}
