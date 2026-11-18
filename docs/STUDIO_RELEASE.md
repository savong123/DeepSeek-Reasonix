---
owner: @esengine
backup: @SivanCola
status: active
reviewed: 2026-11-18
---

# Studio release runbook

Workflow: `.github/workflows/release-studio.yml`. Trigger: push of a `studio-vX.Y.Z` tag, or a `workflow_dispatch` naming an
existing tag.

This is a personal fork of `esengine/DeepSeek-Reasonix`. The release line here produces an **unsigned Windows build** (a
portable zip and an NSIS installer) plus the CLI archives, and publishes them as a GitHub release on this repository.

## 1. What the workflow does and does not do

| Job | Produces | Runner |
| --- | --- | --- |
| `resolve` | Validates the tag shape, resolves the commit and the bare semver, marks a prerelease candidate. | ubuntu-latest |
| `windows` | Builds the SPA and the kernel, then `electron-builder --win zip nsis`. Uploads `studio-windows-amd64`. | windows-latest |
| `cli` | Cross-compiles the CLI for six targets, packages each with the page, writes `SHA256SUMS`. | ubuntu-latest |
| `publish` | Renders the notes, creates or updates the GitHub release with every artifact. | ubuntu-latest |

| Not built here | Why |
| --- | --- |
| macOS and Linux Studio packages | They need Apple Developer ID credentials and a Linux packaging toolchain this fork does not maintain. |
| Authenticode signing | No Certum account. The Windows packages are unsigned and SmartScreen warns. |
| The signed update manifest and the R2 mirror | Both need the upstream object store and the minisign key. |
| npm and Homebrew publishing | Those channels belong to the upstream 1.x line. |

## 2. Preconditions

| ID | Check | Command |
| --- | --- | --- |
| P1 | The release commit is the fetched head of the branch you tag. | `git fetch origin studio && git rev-parse origin/studio` |
| P2 | CI on that commit is green. A flaky job is rerun, not ignored. | `gh run list --branch studio --commit <sha> --json name,conclusion` |
| P3 | `release-notes/studio/X.Y.Z.md` exists in that commit. | `git cat-file -e <sha>:release-notes/studio/X.Y.Z.md` |
| P4 | The version is valid semver and above the latest tag. | `git tag -l 'studio-v*' --sort=-v:refname \| head -1` |
| P5 | Your fork's Actions have enough minutes. A Windows leg costs roughly four times an Ubuntu leg. | GitHub billing page |

Local pnpm MUST be 10. The lockfiles are `lockfileVersion: '9.0'` and a newer major refuses them with
`ERR_PNPM_LOCKFILE_CONFIG_MISMATCH`. Activate it once per machine: `corepack prepare pnpm@10 --activate`.

## 3. Steps

1. Gather the note fragments into `release-notes/studio/X.Y.Z.md` (format in section 6), `git rm` the fragments in the
   same commit, and push.
2. Wait until P2 holds for the commit that contains the notes.
3. Tag the verified commit and push the tag:

   ```bash
   git fetch origin studio
   SHA=$(git rev-parse origin/studio)
   git tag studio-vX.Y.Z "$SHA"
   git fetch origin studio && test "$(git rev-parse origin/studio)" = "$SHA"
   git push origin studio-vX.Y.Z
   ```

4. Watch the run until it finishes:

   ```bash
   RUN=$(gh run list --workflow release-studio.yml --limit 1 --json databaseId --jq '.[0].databaseId')
   gh run watch "$RUN" --exit-status
   ```

5. Verify with section 4 and hand the installer to whoever asked for the build.

A tag that is not `studio-vMAJOR.MINOR.PATCH[-PRERELEASE]` fails `resolve` before anything is built. A prerelease
segment makes the GitHub release a prerelease rather than latest.

## 4. Verification

| ID | Expected | Command |
| --- | --- | --- |
| V1 | The release exists and carries the Windows zip, the installer, six CLI archives and `SHA256SUMS`. | `gh release view studio-vX.Y.Z --json isPrerelease,assets --jq '.isPrerelease, (.assets \| length)'` |
| V2 | A stable tag is not a prerelease; a `-preview.N` tag is. | `gh release view studio-vX.Y.Z --json isPrerelease --jq .isPrerelease` |
| V3 | The body carries the version notes and the standing unsigned-build warning. | `gh release view studio-vX.Y.Z --json body --jq .body` |
| V4 | The installer runs: silent per-user install, the window opens, the kernel starts, then uninstall. | Run `ReasonixStudio-windows-amd64-installer.exe` on a Windows machine. |
| V5 | Every PE image in the build is x64, the shell included. | `node desktop/electron/packaging/pe.js <unpacked-dir> x64` |
| V6 | The CLI archive carries the executable at the root and the page under `frontend-next/dist`. | `unzip -l reasonix-windows-amd64.zip \| head` |

## 5. Recovery

| Symptom | Cause | Action |
| --- | --- | --- |
| No run appears, or a rerun ends in `startup_failure` with no jobs. | GitHub Actions outage. | Wait for queued runs to drain, then `gh workflow run release-studio.yml -f tag=studio-vX.Y.Z`. |
| A build step failed. | Workflow or runner fault. | Fix on `studio` if needed, then dispatch the same tag. The dispatch rebuilds from the tag's commit but uses the workflow from the branch. |
| `resolve` fails on the tag shape. | The tag is not `studio-vX.Y.Z`. | Push a correctly named tag; the old one can be deleted if nothing was published. |
| `publish` fails after the release exists. | Any artifact job failed or the release already had different bytes. | Dispatch the same tag; `publish` re-uploads with `--clobber`. |
| The body is missing or wrong. | Notes are read from the tag's commit, not the branch. | `gh release edit studio-vX.Y.Z --notes-file <file>`. |
| The tag points at the wrong commit and `publish` has not run. | Tagging error. | `git push origin :refs/tags/studio-vX.Y.Z`, delete the local tag, restart at step 3. |
| The tag points at the wrong commit and `publish` has run. | Tagging error after publication. | Do not move the tag. Release the next patch version. |
| SmartScreen blocks the installer. | The build is unsigned. | More info, then Run anyway. Do not report this as a defect. |

## 6. Release note format

File: `release-notes/studio/X.Y.Z.md`, without the tag's `v`. Enforced by `release-note` and `doc-prose`.

| ID | Rule |
| --- | --- |
| R1 | The first line is a one-sentence summary. No other paragraph. |
| R2 | Group headings are `## 新增`, `## 变更`, `## 修复`, `## 移除`, `## 升级须知`, in that order, omitting empty ones. No deeper headings. |
| R3 | One change is one list item within 200 display columns. |
| R4 | Every item names its issue (`#123`), pull request or commit. |
| R5 | Describe what the user observes. The explanation belongs in the linked commit. |
| R6 | The same notes follow in English after a `---` line and the `<!-- lang:en -->` marker, under `## New`, `## Changed`, `## Fixed`, `## Removed`, `## Upgrade notes`; R1 to R5 apply to each language. |

The published body is rendered, not copied: `#N`, a pull request, becomes `#N by @author`, and an issue closed by merged
pull requests becomes `#N fixed in #M by @author`. Preview with
`node scripts/studio-release-notes.mjs release-notes/studio/X.Y.Z.md /tmp/notes.md`; it reads `GH_TOKEN`, else
`gh auth token`.

```markdown
本版修复读图模型误报看不到图，并让被 ACL 残留阻塞的 Windows 安装恢复启动。

## 修复

- 读图模型不再声称看不到已附加的图片 (44150b0aa)
- Windows 安装目录带 AppContainer 包 SID 授权时窗口可以正常打开 #10435
```

## 7. Updating from upstream

The fork tracks `esengine/DeepSeek-Reasonix`'s `studio` branch. Add it once, then merge it like any other remote:

```bash
git remote add upstream https://github.com/esengine/DeepSeek-Reasonix.git
git fetch upstream studio
git merge upstream/studio
```

Conflicts are expected in these two files:

| File | Resolution |
| --- | --- |
| `.github/workflows/release-studio.yml` | Keep this fork's version. |
| `docs/STUDIO_RELEASE.md` | Keep this fork's version. |

Re-read the upstream diff for kernel changes worth carrying over. Never take upstream's workflow wholesale: it
reintroduces the jobs and secrets this fork cannot run.

## 8. Reference

Files this workflow reads:

| Name | Used by |
| --- | --- |
| `scripts/package-cli-archive.sh` | `cli`, to build each archive |
| `scripts/check-cli-archives.py` | `cli`, to reject an incomplete set |
| `scripts/studio-release-notes.mjs` | `publish`, to render the notes with their credits |
| `desktop/electron/packaging/build-host.js` | `windows`, through `pnpm run build:host` |
| `desktop/electron/packaging/canonical-artifact-names.js` | `windows`, to rename `x64` artifacts to `amd64` |

Artifact names, which downstream tooling and this runbook both depend on:

| Artifact | Name |
| --- | --- |
| Portable zip | `ReasonixStudio-windows-amd64.zip` |
| Installer | `ReasonixStudio-windows-amd64-installer.exe` |
| CLI archives | `reasonix-<goos>-<goarch>.zip` or `.tar.gz` |
| Digest list | `SHA256SUMS` |

### Update checks

A build from this fork resolves updates from this repository alone. `internal/platform/update/manifest.go` names
`StudioRepo`, and everything the updater reads is addressed under it:

| What | Address |
| --- | --- |
| Catalog | `releases/latest/download/versions.json` |
| One version's manifest | `releases/download/<tag>/latest.json` |
| One version's notes | `releases/download/<tag>/notes.md` |
| Artifacts and their `.minisig` | `releases/download/<tag>/<artifact>` |

`publish` writes the manifest and the catalog, signs every artifact, and uploads all of it, so a release is what the
updater reads — nothing is fetched from upstream's mirror.

The catalog is rebuilt from the previous release's copy plus the new version, which is why a published release MUST NOT
be deleted: doing so drops its row from every later catalog, and a rollback can no longer reach it.

`internal/platform/update/verify.go` embeds this fork's minisign public key (ID `4F8C82E4ED520BAA`). The private half
lives only in the `MINISIGN_PRIVATE_KEY` and `MINISIGN_PASSWORD` repository secrets. Upstream's key is deliberately not
accepted: a build that took upstream's signatures would let the official channel replace a fork build.
