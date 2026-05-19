# Changelog

Tous les changements notables de ce projet. Format :
[Keep a Changelog](https://keepachangelog.com), SemVer.

## [0.1.0-rc.1] — 2026-05-19

Première prerelease : valide le pipeline de release (goreleaser 6
plateformes + checksums + SBOM Syft + signature cosign keyless). Non
destinée à un usage général.

### Ajouté
- 12 commandes de lecture (Bearer JSON + HTML/CSV plan cookie) :
  `accounts`, `operations`, `transfers`, `budgets`, `incidents`,
  `positions`, `ord-orders`, `ord-fiscalite`, `documents`, `ord-ost`,
  `budget-movements`, `export` — toutes validées sur un compte réel (HTTP 200 +
  données réelles, 2026-05-19) — plus les méta `config` et `version`
  (14 commandes cobra au total).
- Auth `chromecookies` bi-domaine (sans mot de passe, sans secret d'env) +
  amorçage bearer ; sortie agent-first ; transport HTTP audité.
- Outillage de sécurité : `.golangci.yml` (incl. `gosec`), `govulncheck`,
  `Makefile` (`make check` = fmt+vet+test+lint+vulncheck).
- Workflow CI ; `internal/version` + `version`/`--version` ; README,
  CHANGELOG.
- Résilience réseau : backoff throttle (sans ré-auth) vs session bank
  `POST _public_/session/auth/refresh` + un retry — deux boucles séparées ;
  URLs à jeton unique jamais réessayées (`CookieOnce`).
- Distribution Homebrew (parité avec le CLI Go de référence) :
  `scripts/release-homebrew.sh` manuel + playbook
  `docs/releasing-homebrew.md` + template `Formula/boursobank.rb` (copie
  vivante dans un repo `homebrew-tap` séparé). `make homebrew VERSION=x`.
  Volontairement PAS de npm/npx (binaire Go → canaux natifs ; wrapper npm =
  surface supply-chain inutile).
- Release : `.goreleaser.yaml` (6 plateformes, sans CGO, checksums, version
  via ldflags — injection vérifiée), `release.yml` déclenché au tag
  (+ workflow_dispatch) ; `Dockerfile` multi-stage (base Node pour
  chromecookies) avec la limite keychain-hôte documentée ; cibles Makefile
  `release-check`/`snapshot`/`docker`.
- Internationalisation : surface utilisateur en français (public fr) ;
  commentaires/identifiants de code restent en anglais (convention Go).

### Corrigé
- Corrections de schéma trouvées en validation réelle (accounts 47 champs &
  `visibility` bool ; operations/transfers objets imbriqués ; positions
  10 colonnes ; cellules ORD = 3 markups distincts ; budget-movements est
  une liste div pas une table ; export renvoie le CSV directement).
- `gosec` G119 : cookie de session ré-attaché en redirect uniquement vers
  les hôtes boursobank/boursorama autorisés.
- 6 CVE atteignables (5 stdlib + `golang.org/x/net`) → Go 1.25.10 +
  x/net v0.53.0.
- Tests unitaires : client (httptest :
  do/resilientGet/Refresh/Bootstrap/allowlist redirect), config, htmlx,
  out, version, helpers purs cli. La CI plancher les paquets critiques
  75–90 % ; couche commandes assurée par la validation de bout en bout sur compte réel (politique
  honnête documentée, pas de % total truqué).

### Sécurité
- **Bug de fuite de cookie en redirect trouvé par notre propre test &
  corrigé** : `CheckRedirect` *retire* désormais activement le cookie de
  session sur tout hôte non autorisé (Go transmet le header Cookie initial
  aux redirections même-hôte:port-différent).
- Écriture atomique de `config.json`, dossier `0700` / fichier `0600`,
  `user_hash` masqué.
- Vérifié : aucun `.env`/secret dans le dépôt, le CLI ne lit aucun
  identifiant depuis l'environnement.
