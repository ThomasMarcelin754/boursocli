package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/thomasmarcelin754/boursocli/internal/client"
	"github.com/thomasmarcelin754/boursocli/internal/out"
)

// acct holds ONLY the fields needed for account selection + the table view.
// The exhaustive payload is emitted verbatim via out.Raw (lossless), so this
// struct is deliberately minimal and uses only fields whose live type is
// stable (verified against the real account: the full
// object has 47 fields, 7 nullable, and `visibility` is a bool not a string —
// an earlier field list was partly inaccurate; verified, not
// invented). `accountKey` is the 32-hex key (== customId == pfmAccountKey for
// CAV/ORD) used by BOTH the Bearer and cookie planes; `id` (24-hex) is NOT it.
type acct struct {
	AccountKey      string  `json:"accountKey"`
	CustomID        string  `json:"customId"`
	PfmAccountKey   string  `json:"pfmAccountKey"`
	AccountNumber   string  `json:"accountNumber"`
	IBAN            string  `json:"iban"`
	Balance         float64 `json:"balance"`
	Currency        string  `json:"currency"`
	Name            string  `json:"name"`
	Type            string  `json:"type"`         // COMPTE | CCREDIT | ORD
	TypeCategory    string  `json:"typeCategory"` // BANK | CREDITCARD | TRADING
	BankAccountType string  `json:"bankAccountType"`
}

// urlKind maps an account to the cookie-plane URL segment
// (/compte/<kind>/<key>/). Discriminators are the confirmed
// `type` / `typeCategory` (the card account has bankAccountType=null, so we
// must NOT rely on that field alone).
func (a acct) urlKind() string {
	t := strings.ToUpper(a.Type)
	tc := strings.ToUpper(a.TypeCategory)
	bat := strings.ToUpper(a.BankAccountType)
	switch {
	case strings.Contains(bat, "PEA"):
		return "pea" // ORD-analogous path convention (unverified — no PEA held)
	case t == "ORD" || tc == "TRADING" || strings.Contains(bat, "PLACEMENT"):
		return "ord"
	case t == "CCREDIT" || tc == "CREDITCARD":
		return "card"
	case t == "COMPTE" || tc == "BANK" || strings.Contains(bat, "COURANT"):
		return "cav"
	default:
		return ""
	}
}

// resolveAccounts opens a session and returns the live account list (Bearer
// bank/account/accounts) AND the raw body (for lossless JSON output). No
// silent failure: non-200 or empty list is fatal.
func resolveAccounts(ctx context.Context) (*client.Client, []acct, []byte, error) {
	cl, _, _, err := session(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	body, handled, err := getJSON(ctx, cl, "bank/account/accounts")
	if err != nil {
		return nil, nil, nil, err
	}
	if handled {
		return nil, nil, nil, fmt.Errorf("bank/account/accounts : réponse inattendue (non-200 traitée)")
	}
	var accs []acct
	if err := json.Unmarshal(body, &accs); err != nil {
		return nil, nil, nil, fmt.Errorf("décodage bank/account/accounts : %w (corps : %s)", err, snippet(body))
	}
	if len(accs) == 0 {
		return nil, nil, nil, fmt.Errorf("bank/account/accounts a renvoyé une liste vide (≥1 compte attendu) — probablement un souci de session/schéma, pas un état réel")
	}
	return cl, accs, body, nil
}

// pickAccount selects one account by selector: an exact accountKey, or a kind
// (cav|ord|card|pea). Ambiguous or no match ⇒ loud error listing the choices
// (never a silent default).
func pickAccount(accs []acct, selector string) (acct, error) {
	if selector == "" {
		return acct{}, fmt.Errorf("--account manquant : fournir un accountKey ou un type (cav|ord|card|pea). %s", choices(accs))
	}
	var byKey, byKind []acct
	for _, a := range accs {
		if a.AccountKey == selector {
			byKey = append(byKey, a)
		}
		if a.urlKind() == strings.ToLower(selector) {
			byKind = append(byKind, a)
		}
	}
	if len(byKey) == 1 {
		return byKey[0], nil
	}
	switch len(byKind) {
	case 1:
		return byKind[0], nil
	case 0:
		return acct{}, fmt.Errorf("aucun compte ne correspond à %q. %s", selector, choices(accs))
	default:
		return acct{}, fmt.Errorf("%q est ambigu (%d comptes) — préciser un accountKey explicite. %s", selector, len(byKind), choices(accs))
	}
}

func choices(accs []acct) string {
	lines := make([]string, 0, len(accs))
	for _, a := range accs {
		lines = append(lines, fmt.Sprintf("%s=%s (%s)", a.urlKind(), a.AccountKey, a.Name))
	}
	sort.Strings(lines)
	return "Disponibles : " + strings.Join(lines, " · ")
}

// acctsTable renders []acct for the accounts command (table view only).
func acctsTable(accs []acct) out.Table {
	t := out.Table{Cols: []string{"kind", "name", "balance", "currency", "iban", "accountKey"}}
	for _, a := range accs {
		t.Rows = append(t.Rows, []string{
			a.urlKind(), a.Name,
			fmt.Sprintf("%.2f", a.Balance), a.Currency, a.IBAN, a.AccountKey,
		})
	}
	return t
}
