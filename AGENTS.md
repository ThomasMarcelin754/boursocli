# Guide du dépôt — boursobank

CLI agent-first pour un compte BoursoBank **personnel**. Orienté lecture ; le
virement assisté est human-in-the-loop (s'arrête au SCA, n'exécute/contourne
jamais).

## Structure du projet
- `cmd/boursobank/` : point d'entrée (`main.go`), `signal.NotifyContext`.
- `internal/cli/` : arbre cobra, orchestration `session()`, commandes.
- `internal/auth/` : chromecookies bi-domaine (`load.mjs` embarqué) + amorçage bearer.
- `internal/client/` : transport HTTP audité, plans Bearer + cookie.
- `internal/config/` : `config.json` versionné (`CookiesByHost`), `--config`, masquage.
- `internal/out/` : sortie agent-first (JSON stdout, logs stderr, enveloppe ok, table).
- `internal/htmlx/` : parsing HTML strict (dérive de schéma = erreur bruyante).
- `internal/version/` : métadonnées de build (injectables via ldflags).
- Tests : `*_test.go` à côté du code.

## Modèle auth & données (verrouillé)
- Auth = **chromecookies, bi-domaine** (`clients.boursobank.com` + `clients.boursorama.com`, fusionnés, sans filtre de nom) → scrape `DEFAULT_API_BEARER`+`USER_HASH` du dashboard. Requiert Node+npm (install unique `chrome-cookies-secure`) et Chrome connecté à BoursoBank. `--refresh` ré-extrait.
- Données = **Bearer JSON d'abord**, repli HTML/CSV plan cookie. Titres/ORD nécessitent le jar boursorama.
- Sortie = **agent-first** : JSON sur stdout par défaut, diagnostics sur stderr, `--format table` pour les humains, `--quiet`/`--debug`. Sortie 0/1.
- **Secrets jamais loggués** ; `config show` masque ; `config.json` est 0600 et gitignoré.

## Compilation / Tests / Dev
- `go build ./...` · `go run ./cmd/boursobank --help`
- `go test ./...` (ajouter `-race`) ; `go vet ./...`
- `make fmt` = gofmt · `make lint` = golangci-lint (enrichi : bodyclose,gosec,errorlint,errcheck,…) · `make vulncheck` = govulncheck · `make check` = porte complète
- Conventional Commits (`feat: / fix: / docs:`) ; la PR liste les commandes lancées.
- Surface utilisateur en **français** (public fr) ; noms de commandes/flags, identifiants et **commentaires de code restent en anglais** (convention Go).

## Sûreté
- Ne jamais auto-exécuter ni réessayer une étape de virement (non idempotente, sous SCA). Piloter uniquement sur instruction explicite par virement ; s'arrêter à l'écran SCA.
- Outil de compte personnel : sérialiser les requêtes, au rythme humain ; sur throttle Varnish (HTML `401 V`) temporiser, ne pas ré-authentifier.

## État
**12 commandes de lecture construites & validées sur un compte réel** (HTTP 200 + données réelles, 2026-05-19) : `accounts operations transfers budgets incidents positions ord-orders ord-fiscalite documents/releves ord-ost budget-movements export` (+ méta `config`, `version` → 14 commandes cobra au total). Auth = chromecookies bi-domaine → bearer ; résilience réseau (backoff throttle vs refresh de session, séparés). Outillage de production en place : golangci-lint+gosec, govulncheck, CI, goreleaser, Dockerfile, tests unitaires. La validation sur compte réel a **corrigé plusieurs erreurs de schéma** ; les schémas faisant foi sont ceux vérifiés en conditions réelles. **Pas encore construit : le `virement` assisté** (écriture, sous SCA). Ne pas inventer de structures ; s'appuyer sur le comportement vérifié, échouer bruyamment sur dérive.
