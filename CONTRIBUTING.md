# Contributing to nasc

Thanks for your interest in nasc. This project takes patches, bug reports, and ideas.

## Developer Certificate of Origin

nasc uses the [Developer Certificate of Origin](https://developercertificate.org/) (DCO). There is no CLA to sign. Every commit records that you wrote the patch, or otherwise have the right to submit it under the project's Apache-2.0 license.

Sign off each commit:

```bash
git commit -s
```

The `-s` flag appends a `Signed-off-by` line with the name and email from your git config. That line is your agreement to the DCO text below. Set your identity once so the sign-off is accurate:

```bash
git config user.name "Your Name"
git config user.email "you@example.com"
```

If you forget to sign off, amend the last commit with `git commit --amend -s`, or rebase a longer branch with `git rebase --signoff <base>`.

## The DCO

By signing off, you certify the following (the full text lives at developercertificate.org):

> By making a contribution to this project, I certify that the contribution was created in whole or in part by me and I have the right to submit it under the open source license indicated in the file; or the contribution is based upon previous work that is covered under an appropriate open source license and I have the right under that license to submit that work with modifications; or the contribution was provided directly to me by some other person who certified the above and I have not modified it. I understand and agree that this project and the contribution are public and that a record of the contribution (including all personal information I submit with it) is maintained indefinitely.

## Working on the code

nasc targets Go 1.25 and builds with `CGO_ENABLED=0`. Before you open a pull request:

```bash
go build ./...
go vet ./...
go test ./...
```

Every Go source file starts with the two-line SPDX and copyright header used throughout the tree. The header check runs in CI, so keep it on new files.

Golden tests under `internal/cli` compare command output byte for byte against `testdata/golden`. When a change alters that output on purpose, refresh the fixtures with `go test ./internal/cli/ -run TestGolden -update` and review the diff before committing.

## Reporting bugs

Open an issue with the command you ran, what you expected, and what happened. A minimal repository or a snippet of frontmatter that triggers the problem helps a lot.
