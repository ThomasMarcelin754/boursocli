# 🏦 boursocli

CLI agent-first pour un compte BoursoBank (ex-Boursorama Banque)
**personnel**. Orienté lecture ; le virement assisté (prévu) est
human-in-the-loop et **s'arrête à l'écran SCA — il n'exécute ni ne contourne
jamais l'authentification forte**.

> Un seul titulaire, son propre compte, au rythme humain. Pas une ferme de
> scraping, pas multi-locataire. Voir `CLAUDE.md` / `AGENTS.md` pour la
> conception et les règles de contribution.

## Installation

```sh
# Homebrew (macOS/Linux) — build depuis les sources, tire Go + Node :
brew tap thomasmarcelin754/tap
brew install thomasmarcelin754/tap/boursocli

# ou Go :
go install github.com/thomasmarcelin754/boursocli/cmd/boursocli@latest

# ou un binaire de release (goreleaser : darwin/linux/windows × amd64/arm64,
# sans CGO, avec checksums — déclenché au tag via .github/workflows/release.yml)

# ou depuis les sources :
git clone … && cd boursocli && make build && ./boursocli --help
```

Canaux d'installation : Homebrew (tap) · `go install` · binaires de release ·
Docker. **Pas de npm/npx** — un binaire Go se distribue nativement ; un
wrapper npm ajouterait une couche JS + un postinstall qui télécharge un
binaire (surface supply-chain) pour zéro gain. Processus de release Homebrew :
`docs/releasing-homebrew.md`.

**Docker** (`make docker`) : le binaire tourne, mais l'auth chromecookies
déchiffre le keychain de l'OS *hôte* — indisponible dans un conteneur.
À utiliser pour les commandes de lecture avec un `config.json` valide monté :

```sh
docker run --rm -v "$HOME/Library/Application Support/boursocli:/cfg:ro" \
  boursocli:dev --config /cfg/config.json accounts
```
Aucun secret n'est intégré à l'image — les identifiants sont montés au runtime.

Nécessite **Go ≥ 1.21** (avec `GOTOOLCHAIN=auto`, le défaut, Go télécharge
automatiquement la toolchain 1.25.10 requise par `go.mod`). Le chemin d'auth requiert aussi **Node + npm**
une fois (installation unique de `@steipete/sweet-cookie`) et **Chrome
connecté à BoursoBank** — voir Authentification.

## Authentification (sans mot de passe, sans secret d'environnement)

`boursocli` ne demande jamais votre mot de passe et **ne lit aucun secret
depuis l'environnement ni un `.env`**. Il extrait la session BoursoBank
*existante* depuis votre profil **Chrome** local (cookies déchiffrés via le
keychain de l'OS, bi-domaine `clients.boursobank.com` +
`clients.boursorama.com`), puis scrape le bearer API éphémère du dashboard.

- Rester connecté à BoursoBank dans Chrome. Le premier lancement installe un
  helper Node.
- `--refresh` force la ré-extraction des cookies + re-scrape du bearer.
- Les secrets de session vivent **uniquement** dans `config.json` (dossier
  config de l'OS, mode `0600`, dans un dossier `0700`, écriture atomique).
  `config show` masque tout.

### Durabilité de session (espacer les reconnexions)

BoursoBank est sous DSP2/SCA : aucune session n'est éternelle et **aucune
reconnexion ne peut être scriptée** (anti-bot + clavier-image + SCA hors-bande).
On ne *supprime* pas la reconnexion — on l'**espace au maximum** et on la rend
indolore, via le mécanisme prévu par la banque :

1. **Cocher « Se souvenir de moi » à la connexion.** Cela émet le cookie
   `rememberme`, mécanisme *device-trust* prévu par la banque pour
   raccourcir/sauter le SCA d'un navigateur de confiance. ⚠️ *Comportement
   non vérifié empiriquement ici (inféré du flux d'auth) — à confirmer à
   l'usage ; ne pas en dépendre comme d'une garantie.*
2. **Profil Chrome dédié et stable** (utilisé seulement pour BoursoBank, jamais
   nettoyé) : le `rememberme` y survit longtemps. Épinglez-le une fois :
   ```sh
   boursocli config set chrome_profile "Profile 9"   # nom ou chemin
   ```
3. **Sans profil épinglé**, le CLI **auto-sélectionne** le profil dont la
   session BoursoBank est la plus fraîche (scan de tous les profils Chrome,
   métadonnées seules) — fini la loterie entre profils.

Quand la session meurt malgré tout : reconnectez-vous **une fois** dans ce
profil (en cochant « Se souvenir de moi »), puis `--refresh`. C'est le maximum
de durabilité que la DSP2 autorise — aucun raccourci n'existe.

## Utilisation

La sortie est **agent-first** : JSON sur stdout par défaut, diagnostics sur
stderr, code de sortie `0`/`1`. `--format table` pour les humains,
`--quiet`/`--debug`.

```sh
boursocli accounts                      # comptes + soldes (JSON)
boursocli accounts --format table
boursocli operations --account cav      # opés récentes (Bearer, 30 plus récentes)
boursocli export --account cav --out ops.csv   # historique complet CSV
boursocli positions --account ord       # portefeuille titres
boursocli ord-orders --account ord
boursocli ord-fiscalite --account ord --year 2026
boursocli documents --account ord       # relevés / relevés CAV
boursocli ord-ost --account ord
boursocli transfers --account cav
boursocli budgets ; boursocli budget-movements --account cav
boursocli incidents --account cav
boursocli version ; boursocli --version
```

`--account` prend un `accountKey` (32-hex) ou un type : `cav` | `ord` |
`card` | `pea`. Ambiguïté/aucune correspondance → une erreur explicite
listant les choix (jamais un défaut silencieux).

Les 12 commandes de lecture ont été **validées sur un compte réel** (HTTP 200 +
données réelles, 2026-05-19). Les échecs sont toujours explicites : un
non-200, une erreur de décodage ou une dérive de schéma sort en `1` avec
`{"ok":false,"error":…}` — jamais un vide trompeur.

## Cibles Make

```sh
make build   # go build ./...
make test    # go test ./... -race
make lint    # golangci-lint (incl. gosec)
make sec     # lint + govulncheck (sécurité seule)
make check   # fmt vet test lint vulncheck — la porte pré-commit complète
```

## Sécurité

Deux couches, dans `make check` et la CI : **`gosec`** (statique, via
golangci-lint) sur notre code + **`govulncheck`** (scanner CVE officiel de
l'équipe Go, conscient de l'atteignabilité) sur les dépendances et la stdlib.
Aucun secret n'est jamais committé, loggué, ni lu depuis l'environnement.
Modèle complet : `CLAUDE.md` → *Outillage de sécurité*.

## État

Fait : 12 commandes de lecture, sécurité propre, validées sur un compte réel.
Pas encore construit : le `virement` assisté (écriture, sous SCA). Ce dépôt
ne contient que le code du client : pas de spécification d'API tierce.
