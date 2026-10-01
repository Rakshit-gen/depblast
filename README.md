# depblast

Tells you which of your projects pull in a given package version, and
through which chain of dependencies. It reads the lockfiles in your
projects, stores the dependency graph in
[HydraDB](https://github.com/hydra-db/hydradb), and answers questions like
"which of my repos ship semver older than 6, and why?" in one command.

```
$ depblast hit -paths 2 'semver@<6'
microsoft_vscode/package-lock.json
  gulp-json-editor 2.5.0 > js-beautify 1.8.9 > editorconfig 0.15.2 > semver 5.7.2
  gulp 4.0.2 > gulp-cli 2.3.0 > yargs 7.1.2 > read-pkg-up 1.0.1 > read-pkg 1.1.0 > normalize-package-data 2.5.0 > semver 5.7.2

1 copy of semver in 1 of 4 projects
```

The chain tells you what to upgrade: here, `gulp-json-editor`, not
`semver`, is the thing in your package.json.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/Rakshit-gen/depblast/main/install.sh | sh
```

This downloads one archive (about 29 MB) with depblast, the HydraDB server
and the libraries it needs, checks its sha256, unpacks it into
`~/.depblast` and links `~/.local/bin/depblast`. Nothing else needs to be
installed, and there is no server to set up: the first command that needs
HydraDB starts it in the background, and later commands reuse it.

Running the same command again upgrades in place and keeps your data.
`DEPBLAST_VERSION=v0.1.0` installs a given release, `DEPBLAST_HOME` and
`DEPBLAST_BIN_DIR` change where things go. To uninstall, run
`depblast server stop` and delete `~/.depblast` and the link.

Builds exist for macOS on Apple silicon and Linux on x86-64 and arm64
(glibc 2.39 or newer, so Ubuntu 24.04, Debian 13, Fedora 40 or later).
Intel Macs and Windows are not supported.

## Use

```sh
depblast index ~/code                 # every lockfile under ~/code
depblast hit lodash                   # every copy of lodash, and why
depblast hit 'lodash@<4.17.21'        # only versions in a range
depblast hit -json next@16.2.1        # for scripts
depblast audit                        # check everything against OSV.dev
depblast projects                     # what is indexed, and what changed since
```

| Command | What it does |
|---|---|
| `index PATH...` | Finds lockfiles under each folder (skipping `node_modules`, `vendor`, `target`, virtualenvs and hidden folders) and stores their graphs. Lockfiles not modified since they were last indexed are skipped; `-force` stores them anyway. |
| `hit NAME[@VERSIONS]` | Lists every indexed project that locks NAME, with the shortest chain from one of the project's own dependencies to it. `-paths N` shows up to N chains per copy, shortest first. Ranges use `<`, `<=`, `>`, `>=`, `==`, separated by spaces or commas: `'debug@>=2 <2.6.9'`. |
| `audit` | Sends every distinct package version to [OSV.dev](https://osv.dev) and prints the known vulnerabilities with the chain that pulls each one in, grouped by advisory. Aliases (a GHSA and its CVE) are shown once. `-min high` hides lower severities, `-json` for scripts. Exits 3 when it finds anything, so it can fail a CI job. |
| `projects` | Lists indexed lockfiles, marking ones changed since indexing and ones that no longer exist. |
| `forget PATH...` | Removes indexed lockfiles at or under PATH. `-missing` removes the ones that were deleted from disk. |
| `server status\|stop\|log\|rebuild` | Manages the background HydraDB. `rebuild` is explained under Limits. |

Supported lockfiles:

| Ecosystem | Files | Notes |
|---|---|---|
| npm | `package-lock.json`, `npm-shrinkwrap.json` | Lockfile versions 1, 2 and 3. Workspaces count as direct dependencies. |
| Yarn | `yarn.lock` | Classic (v1) only. Reads `package.json` next to it for the direct dependencies. |
| pnpm | `pnpm-lock.yaml` | Lockfile versions 6 and 9. |
| Cargo | `Cargo.lock` | |
| Go | `go.mod` | Runs `go mod graph` and `go list -m all`, so it needs Go installed and may download module metadata. Only the versions Go actually selects are stored. |
| Python | `uv.lock`, `poetry.lock` | Poetry reads `pyproject.toml` for the direct dependencies. |

`audit` sends package names and versions to api.osv.dev, and nothing else.
`index` and `hit` work offline. Full advisory records are cached in
`~/.depblast/osv`.

## How well it works

**Are the chains right?** `scripts/evalnpm` checks `hit` against npm's own
answer. For each lockfile it runs `npm ls --all --package-lock-only` (which
reads the lockfile without installing anything), then asks depblast about
every package in the lockfile and checks three things: depblast finds a
chain exactly when npm lists the package as installed, the chain has the
same length as npm's shortest one, and every link in it is a parent to
child link in npm's tree.

| Lockfile | Packages | Reachable agrees | Shortest length agrees | Every link real |
|---|---|---|---|---|
| jitsi/jitsi-meet | 1611 | 1611 | 1611 | 1611 |
| microsoft/vscode | 1411 | 1411 | 1411 | 1411 |
| mozilla/pdf.js | 832 | 832 | 832 | 832 |
| microsoft/TypeScript | 242 | 242 | 242 | 242 |
| a Next.js site | 122 | 122 | 122 | 122 |

All 4218 agree. They did not at first: the first run found that depblast
skipped the dev dependencies of npm workspace packages, which npm does
install. That is fixed, and covered by a test. The run also turned up two
eval bugs (npm lists aliased packages under the alias, and the TypeScript
checkout needs its workspace folders), both fixed in the eval.

Only npm is checked this way. The other parsers are tested against small
lockfiles written by hand, not against the package managers themselves.

**Speed**, on an Apple M4:

| What | Time | Notes |
|---|---|---|
| `index` of the four repos above, 4096 packages | 0.5 s | Plus about 0.7 s the first time, to start HydraDB. |
| `index` of pdf.js, from nothing installed to done | 0.24 s wall | Includes starting HydraDB. |
| `hit debug` across the four repos | 9 ms median, 111 ms worst | 10 runs, whole CLI process. |
| `hit -paths 5 @babel/traverse` | 9 ms median | Same. |
| `audit` of 3305 distinct versions | 6.5 s | Almost all of it is waiting on OSV.dev, 4 batches of up to 1000. Found 102 advisories. |

## Measured vs Claimed

| Claim | Value | How measured | Date |
|---|---|---|---|
| `hit` chains match `npm ls` | 4218 of 4218 packages | `go run ./scripts/evalnpm` on the five lockfiles above (default branches fetched that day) | 2026-10-02 |
| `hit` latency | 9 ms median | 10 runs of `depblast hit debug`, 4 projects indexed, Apple M4 | 2026-10-02 |
| `index` speed | 4096 packages in 0.5 s | `depblast index` printed time, warm server | 2026-10-02 |
| `audit` time | 6.5 s for 3305 versions | wall time, OSV records cached, home network | 2026-10-02 |
| Download size | 28.8 MB | `depblast-darwin-arm64.tar.gz` from `scripts/bundle.sh` | 2026-10-01 |
| Data folder growth | about 36 MB per re-index of 4096 packages | 10 `index -force` runs: 360.8 MB, then 7.4 MB after `server rebuild` | 2026-10-02 |

## How it uses HydraDB

Each lockfile becomes a `Project` vertex. Each package in it becomes a
`Pkg` vertex with `eco`, `name` and `version` properties, owned by that
project: two projects that both lock `ms 2.0.0` get two vertices, because
npm and pnpm can resolve the same version differently in each tree.
`DEPENDS_ON` edges go from the project to its direct dependencies and from
each package to what it pulled in.

`hit` finds the copies with a property match
(`MATCH (v:Pkg) WHERE v.name = $name`), then for each copy runs HydraDB's
`algo.SPpaths` from the project to the copy, which returns the shortest
paths first. `audit` reads the distinct versions with one query and calls
`hit`'s path query for every vulnerable copy.

Ids are 63-bit hashes of the lockfile path and package, so indexing a
lockfile again replaces its vertices in place. The server is HydraDB's own
`graph-node` binary, run with local file storage under
`~/.depblast/data`, on random localhost ports, with a random token in
`~/.depblast/token`. Set `DEPBLAST_HYDRA_URL` and `DEPBLAST_HYDRA_TOKEN` to
use a HydraDB you run yourself.

## Limits

- **Chains longer than 16 links are not shown.** HydraDB v0.2 caps path
  searches at 16 hops. Such a copy is still listed, without a chain. None
  of the five lockfiles above has one.
- **The data folder only grows.** HydraDB v0.2 keeps replaced data on
  disk, about 36 MB each time 4000 packages are indexed again. `index`
  skips unchanged lockfiles to keep this down, and
  `depblast server rebuild` wipes the folder and indexes every known
  lockfile again.
- **Some direct dependencies are guessed.** npm lockfile version 1 and
  Yarn classic do not say which packages the project itself asked for.
  depblast takes the top level packages that nothing else depends on,
  plus, for Yarn, the names in `package.json`. A package you depend on
  that is also pulled in by something else then shows a chain through
  that something else. The project still ships it, so `hit` still lists it.
- **Version ranges are compared like semver.** That is right for npm,
  Cargo and Go. For Python it is close but not PEP 440: `1.0rc1` and
  `1.0.post1` may sort wrong in a range. Exact versions always work.
- **Go needs Go.** Go modules are read by running the go tool, which can be
  slow the first time while it fetches module metadata.
- **`audit` only knows what OSV.dev knows,** and matches exact versions.

## Building from source

```sh
go build ./cmd/depblast
```

That gives you the CLI, which needs a HydraDB `graph-node` to talk to.
Either set `DEPBLAST_HYDRA_URL` and `DEPBLAST_HYDRA_TOKEN`, or point
`DEPBLAST_GRAPH_NODE` at a `graph-node` built from HydraDB v0.2.0 with
`cargo build --release --features server-runtime --bin graph-node` and
depblast will start it. `scripts/bundle.sh` builds the release archive;
`.github/workflows/release.yml` shows the whole build for each platform.

Tests: `go test ./...`. The tests that need a live server run when
`DEPBLAST_TEST_HYDRA_URL` and `DEPBLAST_TEST_HYDRA_TOKEN` are set
(`internal/store`) or `DEPBLAST_TEST_GRAPH_NODE` is (`internal/server`).

## License

depblast is MIT licensed. The release archives also contain HydraDB, which
is AGPL-3.0; its source is at
<https://github.com/hydra-db/hydradb/tree/v0.2.0> and the archive says so
in `HYDRADB.txt`. depblast runs it as a separate program and does not
modify it.
