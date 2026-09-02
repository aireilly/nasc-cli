# nasc

> **nasc**, *ainmfhocal firinscneach* (masculine noun): genitive singular **naisc**, plural **naisc**
> 1. link, tie, bond
> 2. clasp
> 3. collar (for tethering an animal)
>
> **nasc**, *briathar* (verb): to bind, to tie, to link, to connect. Verbal noun: **nascadh**.
>
> *From Irish (Gaeilge).* 


`nasc` is a CLI that onboards a repository's existing markdown docs for people and AI agents by reading, writing, and enforcing a simple YAML frontmatter.
It marks docs with agent-friendly YAML metadata, generates a docs index for agents, and validates the whole docs corpus against a declared schema.

AI coding agents are expensive at navigating documentation. When docs carry no metadata, an agent falls back on `ls`, `grep`, and `cat`, reading whole files just to find out whether they matter.
With `nasc`, agents can decide what to open in repository, without reading everything first.

Example nasc output [deploy/environments/dev/README.md](https://github.com/aireilly/llm-d-router/pull/1/changes#diff-b9160b43898398c21be85b07a1659f6b5fd93b18df7b8a814e6f48c2aefe2621R1-R19):

```yaml
---
description: Learn how to use dev Kustomize overlays to deploy vLLM in EPD, P/D, E/PD, and fully disaggregated scenarios on KIND or e2e tests.
id: readme
lastUpdated: 2026-07-16
owner: '@llm-d/router-maintainers'
tags:
  - kustomize
  - vllm
  - disaggregation
  - kind
  - e2e
  - overlays
  - deployment
  - dev-environment
title: Development Environment Overlays
type: dev
---
```



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
nasc mark --source file,git --patch
nasc mark --source file,git --write

# Generate the index agents read first.
nasc index --output AGENTS.md

# Gate it in CI. Exit code 3 fails the job.
nasc validate
```

Add an optional LLM source when you want a derived one-line description or tags. `nasc` never vendors an SDK or holds an API key. It shells out to an agent CLI you already run:

```bash
nasc mark --source file,git,llm --llm-cmd "claude -p" --patch
```

### Generating descriptions with LLM

The `description` field is what a human scanning `AGENTS.md`, or an agent choosing what to open, reads first. The `file` and `git` sources cannot write it, because it takes reading the doc to say what the doc teaches. That is the job of the `llm` source.

Run it by adding `llm` to `--source` and passing the CLI to shell out to:

```bash
nasc mark --source file,git,llm --llm-cmd "claude -p" --patch
```

`--llm-cmd` is any command that reads a prompt on stdin and writes a reply to stdout. A bare agent CLI such as `claude -p`, `ollama run <model>`, or `llm` works with nothing wrapped around it, because `nasc` builds the whole prompt itself. For each doc that needs a derived field, `nasc` sends the file path, title, type, and the first 2000 bytes of the body

The prompt tells the agent to write file descriptions in direct active language.

The LLM source is nondeterministic, so `mark` fills a field only when it is absent. A doc that already has a `description` keeps it across runs, and the corpus does not churn. Pass `--force` to regenerate fields that are already present. Set `llm_cmd` and `llm_excerpt_bytes` under `mark:` in `.nasc/config.yaml` to make the command and excerpt size defaults.

### Commands

```bash
nasc schema init [--preset agent-context|minimal]
nasc schema infer [--write]
nasc mark [--source file,git,llm] [--dry-run|--write|--patch] [--llm-cmd <cmd>] [--force]
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

The walker streams markdown paths, honouring `.gitignore` and `.nascignore`, and always skips `.git/`, `.nasc/`, `node_modules/`, and `vendor/`. It also skips files that are agent instructions rather than documentation, so `nasc` never marks or indexes them: `AGENTS.md`, `CLAUDE.md`, `GEMINI.md`, any `SKILL.md`, and everything under a `.claude/` or `.cursor/` directory.

`nasc index --output AGENTS.md` merges rather than overwrites.

## Continuous integration

`nasc validate` reads every doc, checks it against the schema, and exits 3 when a doc breaks a rule. A non-zero exit fails the job, so a pull request that adds an unmarked or malformed doc is caught before it merges.

```yaml
# .github/workflows/docs.yml
name: docs
on: [pull_request]

jobs:
  nasc:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.27"
      - run: go install github.com/aireilly/nasc-cli/cmd/nasc@latest
      - run: nasc validate
```

Add `nasc index --output AGENTS.md --strict` as a second step once a schema is checked in, and the job also fails when a doc is missing schema-required fields. Generate descriptions with the `llm` source in a local `nasc mark` run and commit the result; CI stays deterministic and needs no API key.

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
  llm_excerpt_bytes: 2000   # bytes of each doc sent to the llm; 0 sends the whole file
  llm_prompt: ""          # opening instruction for the llm; empty uses the default below:
  # "You are generating navigation metadata for a documentation file. It serves two
  #  readers at once: a human skimming an index to find the right doc, and an AI agent
  #  deciding whether to load it. Write for both."

index:
  template: templates/agents-index.tmpl
  output: AGENTS.md

output:
  default_format: auto   # auto | table | jsonl
```

`llm_excerpt_bytes` caps how much of each document body reaches the LLM when deriving fields. Set it to `0` to send the whole file.

`llm_prompt` is the opening instruction that frames the task for the model. Leave it unset to use the built-in default.

```yaml
mark:
  llm_cmd: claude -p
  llm_excerpt_bytes: 0
```

## License and attribution

Apache-2.0. See [LICENSE](LICENSE) for the full text.

The convention of writing a doc's `description` to help an agent decide whether to load the doc, rather than as a plain topic summary, was informed by the [code-docs](https://github.com/armstrongl/code-docs) project by Laura Armstrong. No code, configuration, prompt text, or documentation from that project is included in or derived from `nasc`.

Contributions use a DCO. Sign your commits with `git commit -s`.
