# Icon assets

The top-level provider SVGs are unmodified files from [LobeHub Icons](https://icons.lobehub.com), npm `@lobehub/icons-static-svg` version **1.95.1**, MIT license

- `codex-color.svg`, `claude-color.svg`, `deepseek-color.svg`: original color artwork
- `codex.svg`, `grok.svg`, `claude.svg`, `kimi.svg`, `deepseek.svg`, `githubcopilot.svg`: original `currentColor` monochrome artwork
- `LOBEHUB-LICENSE`: upstream MIT notice at git commit `49a2130df7bfa5eb1b088261bff20a37e2967789`

The official [npm archive](https://registry.npmjs.org/@lobehub/icons-static-svg/-/icons-static-svg-1.95.1.tgz) is pinned by SHA-512 integrity `Hw7EPPgVnC4NZLXBfTNJG6hyQgqECfUPC11VVXodPSr1aebKcFxDZlSpxhWwYNdCc6bhxps/x5TtXoPmfKH2ag==`

Interface glyphs in `internal/app/glyphs_gen.go` come from [Remix Icon](https://remixicon.com), npm `remixicon` **4.9.1**, whose included `License` file is **Remix Icon License v1.0 (January 2026)**, preserved in `REMIXICON-LICENSE`; the npm metadata still labels this version Apache-2.0, but the included license file governs this attribution

Regenerate both sets from the repository root with `go run ./Scripts/gen-glyphs`; only generation downloads the pinned official packages, app builds use local embedded assets


`catalog/` holds 110 provider SVG resources for future integrations. Its `manifest.json` records each provider ID, source revision, original filename, and SHA-256. Files are unmodified. The MIT notice for revision `5f93dc3068b3bee3f8753f3772614cc2949c335d` is retained in `catalog/LICENSE`; entries from the pinned LobeHub package use `catalog/LOBEHUB-LICENSE`. Product marks belong to their respective owners.

The catalog uses the Azure AI and Moonshot product marks and GitHub Copilot's robot mark. Alibaba's two plans share artwork; OpenAI API shares the OpenAI knot. Catalog artwork is intended for monochrome template rendering, since some source files use white silhouettes. Only the currently supported providers' top-level files are embedded in the application; preparing a catalog icon does not enable its quota provider.

`go test ./internal/app -run 'TestProviderCatalog|TestEverySelectableProviderHasTrayIcon'` checks the catalog's hashes, SVG decoding, light/dark visibility, and the current providers' menu-bar mappings. Set `RUNWAY_PROVIDER_QA_DIR` to export contact sheets alongside the existing provider previews.
