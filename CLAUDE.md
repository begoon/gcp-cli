# GCP-CLI

Go monorepo of GCP CLI tools. Module name: `gcp`. Go 1.24+.

## Build & Test

```bash
just              # default: test, build, install
just build        # go test ./... && go build (CGO_ENABLED=0, static)
just test         # go test ./...
just install      # copy binaries to ~/bin
just build-amd64  # cross-compile for Linux + UPX compress
```

Individual commands: `go build ./cmd/cr/`, `go test ./cmd/cr/`

## Project Structure

```
cmd/
  cr/       Cloud Run manager (deploy, bounce, revisions, secrets, etc.)
  vm/       GCE VM manager (start/stop, SSH config, ping)
  ver/      Version bumper (VERSION.txt, package.json, pyproject.toml)
  vs/       VS Code remote directory opener
  path/     PATH inspector
  tabber/   Tab writer demo
  try/      Selector demo
lib/
  ext/      Shared utilities (exec, config, UI, colors, notifications)
  completion/zsh/  ZSH completion generation
```

## Key Patterns

- **Config loading**: `ext.LoadVariables()` reads `.cr`, `.env`, `Makefile` (KEY=VALUE format)
- **GCP auth override**: `CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE` in config files
- **Service overrides**: `PROJECTS=svc:project`, `REGIONS=svc:region` (comma-separated)
- **Command execution**: `ext.Exec(cmd, echo)` wraps `script.Exec` with auth override and stderr capture
- **Subcommand dispatch**: `flag.Args()` switch-case pattern, single-letter aliases (e.g., `d` for deploy)
- **Interactive UI**: `survey/v2` for selection, `ext.Selector()` for raw terminal menus
- **Terminal links**: `ext.Href(url, text)` creates OSC 8 clickable hyperlinks
- **Colors**: `ext.Color(text, c.Yellow)` using aurora/v4, imported as `c`
- **Notifications**: `ext.Notify(msg)` — osascript + say (checks ExecutableExists first)
- **ZSH completion**: `zsh.Completion(root)` at command entry, `zsh.NewArg("short:long", "desc")`

## Conventions

- Keep functions short; utility code goes in `lib/ext/`
- Comments lowercase, no period
- Test file: `main_test.go` alongside `main.go`; use `testify/require`
- Flags declared as package-level vars: `var fName = flag.Bool(...)`
- Group related flags in `var (...)` block
- No CGO — all builds use `CGO_ENABLED=0`
- GCP commands use `gcloud` CLI (not API client libraries)
- Quote gcloud args containing `[]` or `()` to prevent shell interpretation
