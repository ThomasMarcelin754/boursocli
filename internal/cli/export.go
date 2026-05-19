package cli

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"time"

	"github.com/spf13/cobra"
	"github.com/thomasmarcelin754/boursobank/internal/out"
)

// exact 11-col CSV header — the contract.
const csvHeader = "dateOp;dateVal;label;category;categoryParent;supplierFound;amount;comment;accountNum;accountLabel;accountbalance"

var csvBOM = []byte{0xEF, 0xBB, 0xBF}

var reDownload = regexp.MustCompile(`https://api\.boursobank\.com/services/api/files/download\.phtml\?token=[^"'<>\s)]+`)

// newExportCmd: cookie-plane operations CSV export chain:
//
//	GET /budget/exporter-mouvements/<accountKey>
//	    ?movementSearch[fromDate]=DD/MM/YYYY&[toDate]=…&[format]=0&[withDocuments]=0
//	→ body carries a one-shot https://api.boursobank.com/services/api/files/
//	  download.phtml?token=… URL → GET it (same cookie) → CSV.
//
// format=0 = CSV (the only one that works directly; 1/2/3 bounce elsewhere).
// The CSV is validated (UTF-8 BOM + the exact 11-col header) — a body that
// isn't that is a loud error, never written silently.
func newExportCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "export",
		Short: "Exporte les opérations en CSV (chaîne à jeton à usage unique, plan cookie)",
	}
	sel := addAccountFlag(c)
	var from, to, outFile string
	c.Flags().StringVar(&from, "from", "", "date de début jj/mm/AAAA (défaut : il y a 3 ans)")
	c.Flags().StringVar(&to, "to", "", "date de fin jj/mm/AAAA (défaut : aujourd’hui)")
	c.Flags().StringVar(&outFile, "out", "", "écrire le CSV ici (défaut : stdout)")
	c.RunE = func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		cl, a, err := resolvePicked(ctx, *sel)
		if err != nil {
			return out.Fail(err)
		}
		now := time.Now()
		if from == "" {
			from = now.AddDate(-3, 0, 0).Format("02/01/2006")
		}
		if to == "" {
			to = now.Format("02/01/2006")
		}
		q := url.Values{}
		q.Set("movementSearch[fromDate]", from)
		q.Set("movementSearch[toDate]", to)
		q.Set("movementSearch[format]", "0") // 0 = CSV, works directly
		q.Set("movementSearch[withDocuments]", "0")
		entry := cookieBase + "/budget/exporter-mouvements/" + a.AccountKey + "?" + q.Encode()

		body, status, err := cl.Cookie(ctx, entry)
		if err != nil {
			return out.Fail(err)
		}
		if status == 401 {
			return out.Fail(fmt.Errorf("entrée export → HTTP 401 (corps vide = probablement throttle Varnish en bordure, PAS une mort d’auth — ralentir et réessayer ; la session est valide). Si ça persiste, se reconnecter dans Chrome"))
		}
		if status != 200 && status != 302 {
			return out.Fail(fmt.Errorf("entrée export → HTTP %d : %s", status, snippet(body)))
		}

		// The server returns the CSV DIRECTLY (BOM body) for format=0. The
		// meta-refresh → download.phtml token is a fallback (older flow).
		// Handle both, prefer the direct body.
		csv := body
		if validateCSV(csv) != nil {
			m := reDownload.Find(body)
			if m == nil {
				return out.Fail(fmt.Errorf("export : la réponse n’est ni un CSV direct ni une redirection download.phtml?token= (statut %d) : %s", status, snippet(body)))
			}
			// One-shot token URL → CookieOnce: a throttled retry would burn
			// the token (regenerate, don't retry the token).
			csv, status, err = cl.CookieOnce(ctx, string(m))
			if err != nil {
				return out.Fail(err)
			}
			if status != 200 {
				return out.Fail(fmt.Errorf("téléchargement export → HTTP %d (le jeton à usage unique a peut-être expiré — relancer, ne pas réessayer le même jeton) : %s", status, snippet(csv)))
			}
			if err := validateCSV(csv); err != nil {
				return out.Fail(err)
			}
		}

		if outFile == "" {
			_, werr := os.Stdout.Write(csv)
			return werr
		}
		if err := os.WriteFile(outFile, csv, 0o600); err != nil {
			return out.Fail(err)
		}
		return out.OK("export", map[string]any{
			"file": outFile, "bytes": len(csv),
			"accountKey": a.AccountKey, "from": from, "to": to,
		})
	}
	return c
}

// validateCSV enforces the documented contract: UTF-8 BOM then the exact
// 11-column header. Anything else (an HTML error page, a truncated body) is a
// loud failure — we never hand back a "successful" non-CSV.
func validateCSV(b []byte) error {
	if !bytes.HasPrefix(b, csvBOM) {
		return fmt.Errorf("export : le corps n’est pas le CSV UTF-8-BOM attendu (obtenu %q…) — probablement une page d’erreur/redirection, pas le fichier", snippet(b))
	}
	rest := b[len(csvBOM):]
	nl := bytes.IndexAny(rest, "\r\n")
	if nl < 0 {
		return fmt.Errorf("export : le CSV n’a pas de ligne d’en-tête")
	}
	if got := string(rest[:nl]); got != csvHeader {
		return fmt.Errorf("export : en-tête CSV non conforme (dérive de schéma).\n obtenu : %s\nattendu : %s", got, csvHeader)
	}
	return nil
}
