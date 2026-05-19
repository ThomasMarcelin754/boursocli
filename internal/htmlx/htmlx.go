// Package htmlx parses BoursoBank cookie-plane HTML strictly. It refuses to
// guess: a selector that matches zero (or an unexpected count of) tables is a
// loud error (schema drift), never a silently-empty result. BoursoBank mixes
// two table systems (modern c-table, legacy Bootstrap responsive with
// hidden-xs/visible-xs dupes) -- we always drop visible-xs cells and let the
// caller map columns by <th> header text.
package htmlx

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Doc wraps a parsed document.
type Doc struct{ q *goquery.Document }

// Parse builds a Doc from an HTML body.
func Parse(body []byte) (*Doc, error) {
	q, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("analyse HTML : %w", err)
	}
	return &Doc{q: q}, nil
}

// Table is one extracted table: <th> header texts + data rows. Each row is the
// list of <td> selections kept after dropping visible-xs mobile duplicates, so
// the caller can pull sub-elements (ISIN span, link label, ...) per cell.
type Table struct {
	Headers []string
	Rows    [][]*goquery.Selection
}

// ExtractOneTable finds exactly one table matching sel. Zero or multiple
// matches => error (schema drift -- never invent/guess). Header row = the <tr>
// containing <th>; data rows = <tr> with <td>, visible-xs cells dropped.
func (d *Doc) ExtractOneTable(sel string) (*Table, error) {
	tables := d.q.Find(sel)
	switch tables.Length() {
	case 1:
	case 0:
		return nil, fmt.Errorf("dérive de schéma : aucune table ne correspond à %q (structure de page modifiée ?)", sel)
	default:
		return nil, fmt.Errorf("dérive de schéma : %d tables correspondent à %q (exactement 1 attendue)", tables.Length(), sel)
	}
	return d.tableFrom(tables.First())
}

// tableFrom builds a Table from a table selection: header row = first <tr>
// with <th>; data rows = <tr> with <td>, visible-xs cells dropped.
func (d *Doc) tableFrom(tbl *goquery.Selection) (*Table, error) {
	t := &Table{}
	tbl.Find("tr").Each(func(_ int, tr *goquery.Selection) {
		if th := tr.Find("th"); th.Length() > 0 && len(t.Headers) == 0 {
			th.Each(func(_ int, s *goquery.Selection) {
				t.Headers = append(t.Headers, clean(s.Text()))
			})
			return
		}
		tds := tr.Find("td")
		if tds.Length() == 0 {
			return
		}
		var row []*goquery.Selection
		tds.Each(func(_ int, td *goquery.Selection) {
			if cls, _ := td.Attr("class"); strings.Contains(cls, "visible-xs") {
				return // mobile duplicate -- drop (per parsing caveat)
			}
			row = append(row, td)
		})
		if len(row) > 0 {
			t.Rows = append(t.Rows, row)
		}
	})
	if len(t.Headers) == 0 {
		return nil, fmt.Errorf("dérive de schéma : la table trouvée n’a pas de ligne d’en-tête <th>")
	}
	return t, nil
}

// Col returns the trimmed text of the cell under header name for a row, or an
// error if the header is unknown or the row has no such index (loud, not a
// silent ""). Use Cell for sub-element access.
func (t *Table) Col(row []*goquery.Selection, header string) (string, error) {
	c, err := t.Cell(row, header)
	if err != nil {
		return "", err
	}
	return clean(c.Text()), nil
}

// Cell returns the *goquery.Selection of the cell under header name.
func (t *Table) Cell(row []*goquery.Selection, header string) (*goquery.Selection, error) {
	for i, h := range t.Headers {
		if h == header {
			if i >= len(row) {
				return nil, fmt.Errorf("la ligne a %d cellules, pas d’index %d pour l’en-tête %q (dérive de schéma)", len(row), i, header)
			}
			return row[i], nil
		}
	}
	return nil, fmt.Errorf("en-tête %q introuvable (présents : %v)", header, t.Headers)
}

// SummaryByLabel reads a label->value map from a summary block (e.g.
// div.c-summary-account-wrapper items, each label in .c-databox__name). The
// value is the item's text minus the label text.
func (d *Doc) SummaryByLabel(itemSel, nameSel string) map[string]string {
	m := map[string]string{}
	d.q.Find(itemSel).Each(func(_ int, it *goquery.Selection) {
		label := clean(it.Find(nameSel).First().Text())
		if label == "" {
			return
		}
		full := clean(it.Text())
		m[label] = clean(strings.TrimPrefix(full, label))
	})
	return m
}

// IsLoginRedirect reports whether the HTML is a logged-out / connexion page
// (so the caller fails with the re-login instruction, not a parse error).
func (d *Doc) IsLoginRedirect() bool {
	h, _ := d.q.Html()
	l := strings.ToLower(h)
	return strings.Contains(l, "/connexion/") &&
		strings.Contains(l, "mot de passe") &&
		d.q.Find("table").Length() == 0
}

// isFRSpace reports the whitespace runes BoursoBank emits in numbers/labels:
// ASCII space/tab/newline/CR, NBSP U+00A0, thin space U+2009, narrow NBSP
// U+202F. Escaped literals only -- no raw special bytes in this source.
func isFRSpace(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\r', '\u00a0', '\u2009', '\u202f':
		return true
	}
	return false
}

// Sel exposes the underlying query for div/li-structured pages that are NOT
// tables (e.g. PFM budget movements — a <ul.list__movement> of
// <li> items, never a <table>).
func (d *Doc) Sel(selector string) *goquery.Selection { return d.q.Find(selector) }

// Clean is the exported text normaliser (callers outside htmlx use it to
// trim cell sub-element text consistently with the parser).
func Clean(s string) string { return clean(s) }

// clean collapses all whitespace (incl. NBSP variants) and trims.
func clean(s string) string {
	s = strings.Map(func(r rune) rune {
		if isFRSpace(r) {
			return ' '
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

// FRClean strips FR-number noise: every space variant, the euro sign
// (U+20AC) and the percent sign; keeps a leading minus; comma->dot.
func FRClean(s string) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case isFRSpace(r) || r == '\u20ac' || r == '%':
			return -1
		case r == '\u2212': // U+2212 MINUS SIGN (BoursoBank uses it, not '-')
			return '-'
		}
		return r
	}, s)
	return strings.TrimSpace(strings.ReplaceAll(s, ",", "."))
}

// FRNumber parses an FR-formatted monetary/percent string to float64. ok=false
// when there is no parseable number (caller decides if that is fatal).
func FRNumber(s string) (float64, bool) {
	c := FRClean(s)
	if c == "" {
		return 0, false
	}
	var f float64
	if _, err := fmt.Sscanf(c, "%g", &f); err != nil {
		return 0, false
	}
	return f, true
}
