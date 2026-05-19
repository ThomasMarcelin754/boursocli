package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRawStr(t *testing.T) {
	if rawStr(json.RawMessage(`"abc"`)) != "abc" {
		t.Fatal("string not unquoted")
	}
	if rawStr(json.RawMessage(`123`)) != "123" {
		t.Fatal("number changed")
	}
	if rawStr(json.RawMessage(`{"a":1}`)) != `{"a":1}` {
		t.Fatal("object should pass through verbatim")
	}
}

func TestDecodeRows(t *testing.T) {
	body := []byte(`{"operations":[{"amount":{"value":1},"label":"x"},{"label":"y"}]}`)
	rows, err := decodeRows(body, "operations")
	if err != nil || len(rows) != 2 {
		t.Fatalf("decodeRows: %v n=%d", err, len(rows))
	}
	if rawStr(rows[0]["amount"]) != `{"value":1}` || rawStr(rows[1]["label"]) != "y" {
		t.Fatalf("rows content wrong: %v", rows)
	}
	if _, err := decodeRows([]byte(`{"x":1}`), "operations"); err == nil {
		t.Fatal("missing array key must be a loud error")
	}
	if _, err := decodeRows([]byte(`not json`), "operations"); err == nil {
		t.Fatal("bad json must error")
	}
}

func TestParseAPIErrAndNotHeld(t *testing.T) {
	e, ok := parseAPIErr([]byte(`{"code":10100,"message":"Ce compte n'existe pas"}`))
	if !ok || e.Code != 10100 {
		t.Fatalf("parseAPIErr: %+v %v", e, ok)
	}
	if _, held := notHeldCodes[e.Code]; !held {
		t.Fatal("10100 must be classified as not-held (skip, not error)")
	}
	if _, ok := parseAPIErr([]byte(`{"operations":[]}`)); ok {
		t.Fatal("a normal payload must not be read as an error envelope")
	}
}

func TestPickAccountAndURLKind(t *testing.T) {
	accs := []acct{
		{AccountKey: "k1", Type: "COMPTE", TypeCategory: "BANK", Name: "CC"},
		{AccountKey: "k2", Type: "ORD", TypeCategory: "TRADING", Name: "Titres"},
		{AccountKey: "k3", Type: "CCREDIT", TypeCategory: "CREDITCARD", Name: "CB"},
	}
	if accs[0].urlKind() != "cav" || accs[1].urlKind() != "ord" || accs[2].urlKind() != "card" {
		t.Fatalf("urlKind: %s %s %s", accs[0].urlKind(), accs[1].urlKind(), accs[2].urlKind())
	}
	if a, err := pickAccount(accs, "k2"); err != nil || a.AccountKey != "k2" {
		t.Fatalf("pick by key: %v %s", err, a.AccountKey)
	}
	if a, err := pickAccount(accs, "ord"); err != nil || a.AccountKey != "k2" {
		t.Fatalf("pick by kind: %v", err)
	}
	if _, err := pickAccount(accs, ""); err == nil {
		t.Fatal("empty selector must error with choices")
	}
	if _, err := pickAccount(accs, "zzz"); err == nil || !strings.Contains(err.Error(), "Disponibles") {
		t.Fatalf("no-match must list choices: %v", err)
	}
	dup := []acct{{AccountKey: "a", Type: "COMPTE"}, {AccountKey: "b", Type: "COMPTE"}}
	if _, err := pickAccount(dup, "cav"); err == nil || !strings.Contains(err.Error(), "ambigu") {
		t.Fatalf("ambiguous kind must error: %v", err)
	}
}

func TestPEAKindAndHelpers(t *testing.T) {
	pea := acct{Type: "COMPTE", BankAccountType: "PEA_PME", Name: "PEA"}
	if pea.urlKind() != "pea" {
		t.Fatalf("PEA kind: %s", pea.urlKind())
	}
	if firstNonEmpty("", "b") != "b" || firstNonEmpty("a", "b") != "a" {
		t.Fatal("firstNonEmpty")
	}
	if snippet([]byte(strings.Repeat("x", 500))) == "" || len(snippet([]byte(strings.Repeat("x", 500)))) != 200 {
		t.Fatal("snippet must cap at 200")
	}
}
