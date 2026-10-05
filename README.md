# GCP CLI

A collection of Go command-line tools for Google Cloud and everyday development tasks. Cloud operations run through the `gcloud` CLI.

| Command | Purpose |
| --- | --- |
| `cr` | Deploy and inspect Cloud Run services and Artifact Registry images |
| `vm` | Manage Compute Engine VMs and update local SSH configuration |
| `ver` | Inspect and update project versions |
| `vs` | Select a local or remote project to open in VS Code |
| `path` | Inspect `PATH`, duplicate directories, and shadowed executables |

## Build and install

Requires Go 1.25 or later. From a local checkout:

```sh
git clone https://github.com/begoon/gcp-cli.git
cd gcp-cli
go test ./...
CGO_ENABLED=0 go build -buildvcs=true -o ./ ./...
mkdir -p ~/bin
cp cr vm ver vs path ~/bin/
```

Add `~/bin` to your `PATH` to run the commands from any directory.

With `just` installed, `just` runs the tests, builds all five binaries, and copies them to `~/bin` (create that directory first). Other recipes:

```sh
just test         # run all Go tests
just build        # test and build locally
just install      # copy existing binaries to ~/bin
just build-amd64  # build for Linux amd64 and compress with UPX
```

Cloud commands require `gcloud` and credentials with access to the relevant resources. `cr metadata` also requires `skopeo`. VM connectivity commands use SSH; `vs` requires the `code` CLI and, for remote projects, VS Code Remote - SSH.

## Cloud Run: `cr`

Create a `.cr` file in your project directory:

```ini
REPO=europe-docker.pkg.dev/my-project/my-repository
NAME=my-image
PROJECT=my-project
REGION=europe-west1
SERVICE=my-service
```

`REPO` and `NAME` form the Artifact Registry image path. Configuration is read from `.cr`, `.env`, and `Makefile` in the current directory, in that order; later files override earlier values. Use plain, unquoted `KEY=VALUE` assignments. These files are parsed as text, with no shell or Make variable expansion.

By default, `cr` uses your existing `gcloud` authentication. To use a credential file, add `CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE=/absolute/path/to/credentials.json` to the configuration.

```sh
cr info             # service links, image, and /health response
cr revisions        # list the 10 most recent registry images
cr deploy           # select an image and confirm deployment
cr wait deploy      # wait for a new image, then select and deploy
cr bounce           # redeploy the current image with a BOUNCED timestamp
cr variables        # show environment variables and secret references
cr -x variables     # also reveal secret values
cr metadata         # select an image and inspect it with skopeo
```

Running `cr` without arguments starts the interactive deployment flow. Put flags before subcommands. Multiple subcommands run in sequence.

| Command | Alias | Description |
| --- | --- | --- |
| `deploy` | `d` | Select and deploy an image to an existing service |
| `create` | `c` | Create a service from a selected image or `-stub` image |
| `bounce` | `b` | Redeploy the current image |
| `info` | `i` | Show service details and check health |
| `health` | `h` | Fetch `/health`, expecting a JSON response |
| `revisions` | `r` | List registry images |
| `wait` | `w` | Wait for a new registry image |
| `metadata` | `m` | Inspect image metadata |
| `variables` | `v` | Show environment variables and secrets |
| `terraform` | `t` | Find related image-tag settings in Terraform files |

`cr create` uses public access (`--allow-unauthenticated`, ingress `all`), port 8000, 512 MiB of memory, one CPU, and zero to one instances. For example:

```sh
cr -stub europe-docker.pkg.dev/my-project/my-repository/my-image:latest create
```

For interactive service selection, set `SERVICE_NAMES=dev-api,prod-api` instead of `SERVICE`. Optional `PROJECTS` and `REGIONS` values provide comma-separated `service:value` overrides:

```ini
SERVICE_NAMES=dev-api,prod-api
PROJECTS=dev-api:my-dev-project,prod-api:my-prod-project
REGIONS=dev-api:europe-west1,prod-api:europe-west2
```

`cr terraform` additionally requires `TF` pointing to a Terraform directory. It searches `main.tf` files containing `# @mark=` and the project name, matching `<name>_image_tag` settings for services named `<environment>-<name>`.

## Compute Engine: `vm`

Create `~/me/vm.json` with a default VM name or alias:

```json
{
  "default": "dev",
  "vms": [
    {
      "project": "my-project",
      "zone": "europe-west1-b",
      "gac": "$HOME/.config/gcp/credentials.json",
      "name": "dev-vm",
      "disk": "/",
      "alias": "dev"
    }
  ]
}
```

`gac` is required and selects the credential file; environment variables such as `$HOME` are expanded. `disk` is the path passed to remote `df -h`. The VM commands expect an external IPv4 address.

For SSH configuration updates, add a matching entry to `~/.ssh/config`, with `HostName` immediately after `Host`:

```sshconfig
Host dev
  HostName 203.0.113.10
  User your-user
```

Ensure `~/Downloads` exists: `vm configure` backs up the SSH configuration there before updating the address.

```sh
vm                 # show the default VM's information
vm -m dev info     # select a VM by name or alias
vm list            # list instances in the selected project
vm start           # start, update SSH configuration, and check connectivity
vm stop            # confirm and stop the VM
vm configure       # update the SSH alias with the current external IP
vm ping            # check SSH connectivity and show remote system information
vm hosts           # list SSH hosts and addresses
vm edit            # edit ~/.ssh/config
vm vm              # edit ~/me/vm.json
```

The edit commands use `$EDITOR`, falling back to `vim`. Use `vm -x info` to include SSH keys in the instance output.

## Version helper: `ver`

Reads `VERSION.txt`, `package.json`, and `pyproject.toml` (both `[project]` and `[tool.poetry]` versions). It processes package metadata at the project root and finds `VERSION.txt` files recursively, excluding paths containing `node_modules`.

```sh
ver                 # display current versions
ver +               # increment the patch version by one
ver +3              # increment the patch version by three
ver 1.2.3           # set an explicit version
ver -p ../project + # operate in another project directory
ver -- -1           # decrement the patch version by one
```

When processing Python metadata, it also updates a matching project entry in `uv.lock` if present in the current working directory.

## VS Code launcher: `vs`

```sh
vs          # select a location, then a project directory
vs github   # select from ~/github, including ~/github/abc projects
vs iproov   # select from ~/iproov
vs vmi      # select a directory on the SSH host vmi
```

The built-in location menu contains `github`, `iproov`, and `vmi`. Other local locations can be supplied as an argument relative to your home directory. Recent selections are stored in `~/.vs_history-*` and shown first.

## PATH inspector: `path`

```sh
path       # list unique PATH directories and executable counts
path -d    # include and mark duplicate directories
path -f    # also list executable names found in multiple directories
```

Missing directories are marked in the output.

## Zsh completion

Add the following to `~/.zshrc` after the binaries are on your `PATH`:

```sh
autoload -Uz compinit
compinit
eval "$(cr completion)"
eval "$(vm completion)"
```

## License

[MIT](LICENSE). Copyright (c) 2025 Alexander Demin.
