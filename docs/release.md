# Release

Tagged releases use GoReleaser via `.github/workflows/release.yml`.
Configuration lives in `.goreleaser.yaml`.

## Homebrew

The Homebrew formula is published to `corca-ai/homebrew-tap`.
Keep the `brews` entry pointed at `directory: Formula`; the shared tap uses
`Formula/` as its canonical formula directory, and root-level formula files are
ignored once that directory exists.
