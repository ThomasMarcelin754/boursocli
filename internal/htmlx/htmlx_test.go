package htmlx

import "testing"

func TestFRNumber(t *testing.T) {
	cases := []struct {
		in   string
		want float64
		ok   bool
	}{
		{"1\u00a0234,56\u00a0\u20ac", 1234.56, true}, // NBSP thousands + euro
		{"1\u202f234,56\u00a0\u20ac", 1234.56, true}, // narrow NBSP
		{"-240,00", -240, true},                      // negative
		{"1\u2009000", 1000, true},                   // thin space
		{"+12,34\u00a0%", 12.34, true},               // percent + plus
		{"", 0, false},                               // empty
		{"\u00a0\u20ac\u00a0", 0, false},             // no digits
		{"3\u00a0456,7", 3456.7, true},               // ASCII/NBSP mix
	}

	for _, c := range cases {
		got, ok := FRNumber(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("FRNumber(%q) = %v,%v; want %v,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestExtractOneTableDrift(t *testing.T) {
	d, err := Parse([]byte(`<html><body><p>no table here</p></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.ExtractOneTable("table.c-table"); err == nil {
		t.Fatal("expected loud schema-drift error when no table matches, got nil")
	}
}

func TestExtractOneTableMultipleIsDrift(t *testing.T) {
	d, _ := Parse([]byte(`<table class="x"><tr><th>A</th></tr><tr><td>1</td></tr></table>
	<table class="x"><tr><th>B</th></tr><tr><td>2</td></tr></table>`))
	if _, err := d.ExtractOneTable("table.x"); err == nil {
		t.Fatal("2 matching tables must be a loud schema-drift error")
	}
}

func TestColCellErrorsAndSummary(t *testing.T) {
	html := `<div class="wrap"><div class="item"><span class="nm">Total</span> 1 234,56 €</div></div>
	<table class="t"><tr><th>A</th><th>B</th></tr><tr><td>x</td><td><span class="z">y</span></td></tr></table>`
	d, _ := Parse([]byte(html))
	tbl, err := d.ExtractOneTable("table.t")
	if err != nil {
		t.Fatal(err)
	}
	row := tbl.Rows[0]
	if v, _ := tbl.Col(row, "A"); v != "x" {
		t.Fatalf("Col A = %q", v)
	}
	if _, err := tbl.Col(row, "NOPE"); err == nil {
		t.Fatal("unknown header must error, not return empty")
	}
	cell, err := tbl.Cell(row, "B")
	if err != nil || cell.Find("span.z").Text() != "y" {
		t.Fatalf("Cell sub-element access broken: %v", err)
	}
	if m := d.SummaryByLabel(".item", ".nm"); m["Total"] == "" {
		t.Fatalf("SummaryByLabel did not extract value: %v", m)
	}
}

func TestIsLoginRedirect(t *testing.T) {
	dead, _ := Parse([]byte(`<html><body>/connexion/ mot de passe</body></html>`))
	if !dead.IsLoginRedirect() {
		t.Fatal("logged-out page not detected")
	}
	live, _ := Parse([]byte(`<table><tr><td>data</td></tr></table>`))
	if live.IsLoginRedirect() {
		t.Fatal("a page with a table is not a login redirect")
	}
}

func TestExtractOneTableHeadersAndVisibleXS(t *testing.T) {
	html := `<table class="t"><tr><th>A</th><th>B</th></tr>
	<tr><td class="hidden-xs">a1</td><td class="visible-xs">dup</td><td>b1</td></tr></table>`
	d, _ := Parse([]byte(html))
	tbl, err := d.ExtractOneTable("table.t")
	if err != nil {
		t.Fatal(err)
	}
	if len(tbl.Headers) != 2 || tbl.Headers[0] != "A" {
		t.Fatalf("headers = %v", tbl.Headers)
	}
	if len(tbl.Rows) != 1 || len(tbl.Rows[0]) != 2 {
		t.Fatalf("visible-xs not dropped: row = %d cells", len(tbl.Rows[0]))
	}
	v, err := tbl.Col(tbl.Rows[0], "B")
	if err != nil || v != "b1" {
		t.Fatalf("Col(B) = %q, %v", v, err)
	}
}
