# CLAUDE.md — boursobank

Repères pour Claude Code / les agents travaillant dans ce dépôt. Les
conventions pour contributeurs humains sont dans `AGENTS.md` ; ce fichier est
la carte d'orientation. Lire les deux.

## De quoi il s'agit

CLI Go agent-first pour un compte BoursoBank (ex-Boursorama Banque)
**unique, personnel**. Orienté lecture. Le virement assisté est
**human-in-the-loop** : il pilote l'assistant de virement mais **s'arrête à
l'écran SCA et ne l'exécute ni ne le contourne jamais**. Pas un outil
multi-locataire, pas une ferme de scraping — un titulaire, son propre
compte, au rythme humain.

## ⚠️ Dépôt public — aucun secret, jamais

Ce CLI est destiné à être **public**. Règles dures :

- **Ne jamais committer** un cookie, bearer/JWT, `userHash`, IBAN, clé de
  compte, solde, ou toute valeur identifiant le compte. Ni dans le code, les
  tests, les fixtures, les docs ou les messages de commit. Utiliser des
  placeholders évidents (`<accountKey>`, `Bearer <token>`).
- Les secrets de runtime vivent **uniquement** dans `config.json` (dossier
  config de l'OS, mode `0600`, gitignoré) — jamais dans l'arbre du dépôt,
  jamais dans `.env`.
- `config show` doit rester masqué. Les diagnostics vont sur **stderr** ; ne
  jamais logguer un secret, même en `--debug`.
- Le détail des endpoints, schémas de réponse et internes d'auth n'a pas sa
  place dans ce dépôt public : il ne contient que le **code du client**, pas
  de spécification d'API tierce. Ne pas coller de telles specs ici.

## Architecture (code actuel)

```
cmd/boursobank/main.go     point d'entrée; signal.NotifyContext (SIGINT/SIGTERM); sortie 0/1
internal/cli/root.go       arbre cobra + flags persistants + orchestration session()
internal/cli/accounts.go   `config show`, `accounts`
internal/auth/             extraction chromecookies bi-domaine (+ load.mjs embarqué)
internal/client/client.go  transport HTTP audité; amorçage bearer; 3 plans
internal/config/config.go  config.json versionné (CookiesByHost), 0600, Redacted()
internal/out/out.go        sortie agent-first (JSON stdout / logs stderr / table)
```

Flags (persistants) : `--config --chrome-profile --format(json|table)
--quiet --debug --refresh`.

## Modèle d'auth (acté)

BoursoBank s'étend sur **deux domaines enregistrables** :
`clients.boursobank.com` et `clients.boursorama.com`. La banque fonctionne
avec le seul jar boursobank ; titres/ORD/bourse **nécessitent** aussi le jar
boursorama (posé via le SSO `x-domain-authentification`).

Flux `session()` :

1. **Cookies** : `internal/auth` lance un helper Node embarqué (`load.mjs`)
   qui lit le magasin de cookies **Chrome** local via `chrome-cookies-secure`
   — déchiffré par le keychain de l'OS, DB copiée en dossier temp pour
   fonctionner Chrome ouvert. Lancé **une fois par domaine**, jars fusionnés,
   **sans filtre de nom** (capture tout). L'utilisateur reste juste connecté
   à BoursoBank dans Chrome. `npm install chrome-cookies-secure` unique dans
   un cache ; nécessite Node+npm.
2. **Amorçage bearer** : GET du dashboard avec le cookie fusionné, scrape
   `DEFAULT_API_BEARER` (~24 h) + `USER_HASH` du HTML.
3. Sur session Chrome morte (302→login / pas de bearer) : **une** ré-extraction
   automatique des cookies + retry. `--refresh` force ré-extraction + re-scrape.

Pas de `.env`, pas de collage manuel de cookie, pas de loopback OAuth
(inapplicable à Bourso).

## Plans de données

- **Bearer JSON (préféré)** — `api.boursobank.com/.../_user_/_<hash>_/<resource>`,
  `Authorization: Bearer …`, aucun cookie envoyé.
- **Plan cookie (repli)** — HTML/CSV `clients/bourse.boursobank.com` avec le
  cookie bi-domaine fusionné. Titres/ORD nécessitent le jar boursorama.

Le transport est audité et **verrouillé** : proxy-from-env, TLS ≥ 1.2,
**HTTP/2 off** (certains fronts Bourso bloquent en H2 depuis Go), timeout
20 s, **pas de cookie jar** (le header `Cookie` fusionné est posé
manuellement — exigence bi-domaine), et un `CheckRedirect` qui ré-attache
le cookie+UA **uniquement vers les hôtes boursobank/boursorama autorisés**
(Go retire le Cookie en cross-host ; un 302 hostile ne doit jamais
exfiltrer la session).

## Résilience réseau (deux boucles SÉPARÉES — `client.resilientGet`)

Les deux classes d'échec ont des remédiations différentes et ne doivent pas
être confondues :

- **Throttle** (bordure Varnish : HTML `401 V`/`Not Authorized`, ou `503`) —
  la session est saine. Backoff exponentiel + jitter (2/4/8 s, conscient du
  ctx), **pas de ré-auth**, max 3 essais.
- **Expiration de session bank** (plan bearer `{"code":10006}` ou `401 JWT
  Token not found`) — `POST _public_/session/auth/refresh` (renouvelle côté
  serveur SANS re-scraper le dashboard) puis **un** retry. Si le refresh
  échoue, la réponse est renvoyée pour que la commande émette l'instruction
  bruyante de reconnexion. Le 10006 sur `trading/*` n'est PAS ça (c'est
  l'élévation bourse) — la commande affiche alors l'indication actionnable.

`API`/`Cookie` sont résilients (GET idempotents). Les URLs à usage unique
(`download.phtml?token=`) utilisent `CookieOnce` — **jamais** réessayées,
sinon un retry sous throttle brûlerait le jeton.

## État

- ✅ Auth vérifiée en live : chromecookies bi-domaine → bearer.
- ✅ **12 commandes de lecture construites & validées sur un compte réel**
  (HTTP 200 + données réelles, 2026-05-19) : `accounts`, `operations`, `transfers`,
  `budgets`, `incidents`, `positions`, `ord-orders`, `ord-fiscalite`,
  `documents`/relevés, `ord-ost`, `budget-movements`, `export` — plus les
  méta `config` et `version` (14 commandes cobra au total).
- ✅ Reconnexion propre (`session/auth/refresh`) + taxonomie des codes 401
  (backoff throttle vs refresh session bank, boucles séparées) — implémenté.
- ⬜ Pas encore construit : le `virement` assisté (écriture, sous SCA —
  s'arrêter au SCA, ne jamais exécuter/contourner). S'appuyer sur le
  comportement de l'API vérifié sur compte réel ; ne pas inventer de
  structures ; échouer bruyamment sur dérive.
- ✅ Outillage : Makefile, README, CHANGELOG, CI, golangci-lint+gosec,
  govulncheck, binaire versionné, tests unitaires, goreleaser (6
  plateformes) + release au tag, Dockerfile.

## Politique de tests & couverture (honnête par conception)

On ne court **pas** après un % de couverture total truqué. La CI impose des
planchers sur les paquets unit-testables de correction/sécurité/parsing —
`client` (transport, allowlist cookie redirect, résilience), `config`
(écriture atomique 0600, masquage), `htmlx` (parsing strict, nombres FR),
`out`, `version` — à 75–90 %. La couche commandes (glue `internal/cli`) et
`internal/auth` (lance Node pour chromecookies — non mockable en unit sans
falsifier l'objet même du test) sont assurées par la **validation de bout
en bout sur le compte réel, documentée** : les 12 commandes de lecture ont
tourné contre le compte réel, HTTP 200 + données réelles, schéma vérifié
(2026-05-19). Un 75 % mocké superficiel serait *moins* d'assurance que ça,
pas plus — la porte reflète donc ce qui a un sens réel.

## Compilation / tests

```
make build   # go build ./...
make test    # go test ./... -race
make vet     # go vet ./...
make fmt     # gofmt -w internal cmd
make lint    # golangci-lint run ./...
make sec     # lint + vulncheck (sécurité seule)
make check   # fmt vet test lint vulncheck — la porte pré-commit complète
```

Conventional Commits. Trailer sur les commits comme configuré pour cet
environnement.

## Outillage de sécurité (deux couches, dans `make check`)

C'est un CLI publié qui manipule une session bancaire personnelle — la
sécurité est gatée, pas optionnelle.

1. **Analyse statique — `golangci-lint`** (`.golangci.yml`). L'« ESLint » de
   Go : un runner de 100+ linters. On lance le set standard (errcheck,
   govet, staticcheck, ineffassign, unused) **plus** un set enrichi incl.
   **`gosec`** (le scanner de sécurité Go : creds en dur, crypto faible,
   perms fichiers, injection commande/chemin, headers sensibles en
   redirect, …), `bodyclose`, `errorlint`, `errchkjson`, `nilerr`. C'est
   `gosec` qui a attrapé le risque d'exfiltration de cookie en redirect
   (G119, désormais host-allow-listed). Statique seulement — analyse *notre*
   code, pas les CVE des dépendances. (`misspell` volontairement désactivé :
   anglais-only, faux positifs sur la surface française.)

2. **Scan CVE connues — `govulncheck`** (`make vulncheck`). L'outil officiel
   de l'équipe Go (`golang.org/x/vuln`), adossé à la base de vulnérabilités
   Go curée (NVD + GitHub Advisory + mainteneurs, format OSV). **Conscient
   de l'atteignabilité** : ne signale une vulnérabilité que si notre code
   *appelle* transitivement le symbole vulnérable — peu de bruit. Couvre les
   modules tiers **et la stdlib / toolchain Go** (la plupart des trouvailles
   ici étaient stdlib → corrigées en montant la directive `go`). Sortie
   3 = vulnérable, 0 = propre.

Ni l'un ni l'autre ne fait de test dynamique/runtime (pas de DAST, pas de
fuzzing) — les deux sont du SAST. Garder la version `go` de `go.mod` à jour :
une toolchain périmée est le hit `govulncheck` le plus courant. La CI doit
lancer `make check` et échouer sur non-zéro.

## Règles de sûreté dures

- **Ne jamais** auto-exécuter, auto-réessayer ni contourner une étape de
  virement. C'est non idempotent et sous SCA. N'agir que sur instruction
  explicite par virement du titulaire ; **s'arrêter à l'écran SCA** et rendre
  la main à l'humain.
- Compte personnel : **sérialiser** les requêtes, au rythme humain. Sur
  throttle Varnish (HTML `401 V`) **temporiser** — **ne pas** ré-authentifier
  en boucle.
- Toujours nettoyer les fichiers temp contenant IBAN/jetons. Aucun secret
  dans un artefact committé (voir la section dépôt public).
