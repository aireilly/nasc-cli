# nasc CLI

> **nasc**, *ainmfhocal firinscneach* (masculine noun): genitive singular **naisc**, plural **naisc**
> 1. link, tie, bond
> 2. clasp
> 3. collar (for tethering an animal)
>
> **nasc**, *briathar* (verb): to bind, to tie, to link, to connect. Verbal noun: **nascadh**.
>
> *From Irish (Gaeilge).* 


`nasc` onboards a repository's existing markdown docs for AI agent consumption and enforces a simple YAML front matter in CI.
It marks docs with agent-friendly YAML metadata, generates a docs index for agents, and validates the whole docs corpus against a declared schema.

AI coding agents are expensive at navigating documentation. When docs carry no metadata, an agent falls back on `ls`, `grep`, and `cat`, reading whole files just to find out whether they matter.
With `nasc`, agents can decide what to open in repository, without reading everything first.

## Install

Requires Go 1.25 or newer.

```bash
go install github.com/aireilly/nasc-cli/cmd/nasc@latest
```

Or build from a clone:

```bash
git clone https://github.com/aireilly/nasc-cli
cd nasc-cli
go build -o nasc ./cmd/nasc
```

## Quick start

`mark`, `index`, and `validate` work with no configuration. When there is no `.nasc/schema.yaml`, nasc uses a built-in `agent-context` default. Run `nasc schema init` only when you want to pin or change that schema in the repo.

```bash
# Optional: pin a schema in the repo so the rules are explicit and reviewable.
nasc schema init --preset agent-context

# Look at what you already have, and let nasc propose a schema.
nasc schema infer --write

# Backfill metadata from file facts and git history. Review the patch before writing.
nasc mark --tier file,git --patch
nasc mark --tier file,git --write

# Generate the index agents read first.
nasc index --output AGENTS.md

# Gate it in CI. Exit code 3 fails the job.
nasc validate
```

Add an optional LLM tier when you want a derived one-line description or tags. `nasc` never vendors an SDK or holds an API key. It shells out to an agent CLI you already run:

```bash
nasc mark --tier file,git,llm --llm-cmd "claude -p" --patch
```

### Commands

```bash
nasc schema init [--preset agent-context|minimal]
nasc schema infer [--write]
nasc mark [--tier file,git,llm] [--dry-run|--write|--patch] [--llm-cmd <cmd>] [--force]
nasc index [--output <file>] [--template <file>] [--strict]
nasc validate [--severity error|warn] [--json]
nasc get <path> [key]
nasc doctor
```

```bash
$ nasc --help
Onboard a repository's docs for AI agents and enforce that markup in CI.

Usage:
  nasc [flags]
  nasc [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  doctor      Report the environment nasc sees.
  get         Read one frontmatter value from a document.
  help        Help about any command
  index       Generate an AGENTS.md-style navigation index.
  mark        Derive agent-navigation metadata and write it into frontmatter.
  schema      Inspect, create, or infer a schema.
  validate    Enforce the schema across the corpus. Fails CI on error findings.
```


## How it works

`nasc` runs a three-stage in-memory pipeline on every invocation:

```
walk repo → parse each doc (frontmatter + body) → one of { mark | index | validate }
```

The walker streams markdown paths, honouring `.gitignore` and `.nascignore`, and always skips `.git/`, `.nasc/`, `node_modules/`, and `vendor/`. Parsers produce a `Doc` per file. `nasc` then reads and updates the repo markdown as required.

## Exit codes

| Code | Meaning                                                       |
| ---- | ------------------------------------------------------------- |
| 0    | success, corpus is agent-ready                                |
| 1    | success, nothing to do (empty result, empty diff)             |
| 2    | usage error                                                   |
| 3    | validation failure (`validate`, `index --strict`)             |
| 5    | write conflict, a file changed under us during `mark --write` |

## Configuration

`.nasc/config.yaml`, every key optional:

```yaml
root: .
include: ["**/*.md", "**/*.mdx"]
exclude: ["CHANGELOG.md", "node_modules/**"]

mark:
  llm_cmd: ""
  llm_excerpt_bytes: 2000

index:
  template: templates/agents-index.tmpl
  output: AGENTS.md

output:
  default_format: auto   # auto | table | jsonl
```

## License and attribution

Apache-2.0. See [LICENSE](LICENSE) for the full text.

The convention of writing a doc's `description` as a load-or-skip trigger, rather than a topic summary, was informed by the [code-docs](https://github.com/armstrongl/code-docs) project by Laura Armstrong. No code, configuration, prompt text, or documentation from that project is included in or derived from `nasc`.

Contributions use a DCO. Sign your commits with `git commit -s`.
