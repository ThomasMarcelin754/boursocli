# boursocli — playbook de release Homebrew

Distribution Homebrew légère (parité avec le CLI Go de référence) : la
**formule vit dans un repo tap séparé**, ce dépôt ne porte que le script
helper + ce playbook + le template de formule (`Formula/boursocli.rb`).
Pas de npm/npx — un binaire Go se distribue nativement via
Homebrew/`go install`/binaires de release (npm impliquerait un wrapper JS +
un postinstall qui télécharge un binaire = surface supply-chain en plus pour
~0 gain ; volontairement non fait).

## 0) Prérequis
- Arbre git propre sur `main`, le tag de release poussé (ex. `v0.1.0`).
- Un repo tap `thomasmarcelin754/homebrew-tap` existe (un repo GitHub *séparé*,
  nommé `homebrew-tap`) avec `Formula/boursocli.rb` initialisé depuis le
  template `Formula/boursocli.rb` de ce dépôt.

## 1) Générer les champs de la formule
```sh
scripts/release-homebrew.sh 0.1.0      # ou : make homebrew VERSION=0.1.0
```
Copier les `version`, `url`, `sha256` affichés.

## 2) Mettre à jour le tap
Éditer `../homebrew-tap/Formula/boursocli.rb` :
- mettre `version "X.Y.Z"`
- mettre `url "https://github.com/thomasmarcelin754/boursocli/archive/refs/tags/vX.Y.Z.tar.gz"`
- coller `sha256 "…"`

```sh
git -C ../homebrew-tap commit -am "boursocli vX.Y.Z"
git -C ../homebrew-tap push origin main
```

## 3) Vérifier l'installation
```sh
brew uninstall boursocli || true
brew untap thomasmarcelin754/tap || true
brew tap thomasmarcelin754/tap
brew install thomasmarcelin754/tap/boursocli
brew test thomasmarcelin754/tap/boursocli
boursocli --version
```

## Notes
- La formule **build depuis les sources** (`depends_on "go" => :build`) et
  injecte la version via `-ldflags` (mêmes variables que `.goreleaser.yaml`).
- **Runtime** : le chemin d'auth (chromecookies) lance `node`/`npm`, donc la
  formule `depends_on "node"`. Sans Node, seul un `config.json` déjà
  rempli fonctionne (commandes de lecture) — documenté dans le README.
- C'est volontairement manuel (pas de bloc goreleaser `brews:`, pas de
  token auto) : moins de pièces mobiles, aucun secret CI, conforme à
  l'approche du CLI de référence.
