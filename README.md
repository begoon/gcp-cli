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

Requires Go 1.27 or later. From a local checkout:

```sh
git clone https://github.com/begoon/gcp-cli.git
cd gcp-cli
go test ./...
CGO_ENABLED=0 go build -buildvcs=true -o xt ./cmd/xt
mkdir -p ~/bin
cp xt ~/bin/
for name in cr vm ver vs path; do ln -sfn xt ~/bin/"$name"; done
```

Add `~/bin` to your `PATH` to run the commands from any directory.

All tools share one executable, `xt`. Run `xt cr info`, `xt vs`, or `xt path -f`; the symlinks also let you keep using `cr info`, `vs`, and `path -f`. Flags follow the tool name: `xt vm -m dev info`.

With `just` installed, `just` runs the tests, builds `xt`, installs it to `~/bin`, and creates the command symlinks. Other recipes:

```sh
just test                  # run all Go tests
just build                 # test and build xt locally
just install               # copy existing xt to ~/bin
just install-links         # install xt and create/update all command symlinks
just build-amd64            # build Linux amd64 xt and compress with UPX
just install-remote         # copy amd64/xt to vmi:~/.local/bin
just install-remote-links   # copy amd64/xt and create/update remote command symlinks
VM=my-host just install-remote-links
```

Link recipes replace existing command binaries or symlinks. Remote installation defaults to SSH host `vmi`; set `VM` to choose another host, and add `~/.local/bin` to its `PATH`.

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

For services with native Cloud Run IAP enabled, `cr` shows the Cloud Run readiness condition and skips `/health` unless an IAP service account is configured. Readiness does not verify application health. To authenticate the health request, add this to `.cr` or `.env`:

```ini
IAP_SERVICE_ACCOUNT=health-checker@my-project.iam.gserviceaccount.com
```

Or run `cr -iap-service-account health-checker@my-project.iam.gserviceaccount.com info`. The setting also enables authenticated checks when IAP is not advertised in the service metadata. `cr` uses `gcloud iam service-accounts sign-jwt` to create a short-lived JWT scoped to the health URL and sends it in `Proxy-Authorization`. This supports Google-managed IAP OAuth clients without a client ID or downloaded service-account key. Your gcloud identity needs `roles/iam.serviceAccountTokenCreator` on that service account, and the service account needs `roles/iap.httpsResourceAccessor` on the IAP resource. See [IAP programmatic authentication](https://docs.cloud.google.com/iap/docs/authentication-howto).

To create the account and allow it to access all current and future IAP-protected web services in a project, replace `my-project` and `you@example.com` below. Run these commands using a gcloud identity with permission to create service accounts and update the relevant IAM policies:

```sh
IAP_PROJECT=my-project
IAP_USER=you@example.com
IAP_SA="health-checker@${IAP_PROJECT}.iam.gserviceaccount.com"

gcloud iam service-accounts create health-checker \
  --project="$IAP_PROJECT" \
  --display-name="IAP health checker"

gcloud iam service-accounts add-iam-policy-binding "$IAP_SA" \
  --project="$IAP_PROJECT" \
  --member="user:$IAP_USER" \
  --role=roles/iam.serviceAccountTokenCreator \
  --condition=None

gcloud projects add-iam-policy-binding "$IAP_PROJECT" \
  --member="serviceAccount:$IAP_SA" \
  --role=roles/iap.httpsResourceAccessor \
  --condition=None
```

Token-signing permission is scoped to this service account; IAP access is project-wide. Allow a few minutes for IAM changes to propagate before running `cr info`.

Health requests have a 30-second timeout and do not follow redirects. Authentication failures, HTTP errors, and non-JSON responses are reported without terminating `info` or `bounce`; health failures retain the existing non-fatal behavior.

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
vs vmi      # select a directory on the SSH host vmi
```

The built-in location menu includes `github` and `vmi`. Other local locations can be supplied as an argument relative to your home directory. Recent selections are stored in `~/.vs_history-*` and shown first.

## PATH inspector: `path`

```sh
path       # list unique PATH directories and executable counts
path -d    # include and mark duplicate directories
path -f    # also list executable names found in multiple directories
```

Missing directories are marked in the output.

## Shell completion

For Bash, add this to `~/.bashrc` after `xt` is on your `PATH`:

```sh
eval "$(xt completion bash)"
```

This completes tool names after `xt`, and Cloud Run/VM subcommands and aliases for both `xt cr` / `xt vm` and the `cr` / `vm` symlinks. It does not query cloud resources or complete flag values.

For only the standalone commands, use `eval "$(cr completion bash)"` and `eval "$(vm completion bash)"`.

### Zsh

Add the following to `~/.zshrc` after the binaries are on your `PATH`:

```sh
autoload -Uz compinit
compinit
eval "$(xt completion zsh)"
```

This registers completion for `xt` and the `cr` / `vm` symlinks. For standalone setup, `cr completion zsh` and `vm completion zsh` are also available.

The existing `completion` command defaults to Zsh; `completion zsh` is also accepted.

## License

[MIT](LICENSE). Copyright (c) 2025 Alexander Demin.
