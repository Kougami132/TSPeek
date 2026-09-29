## Agent skills

### Issue tracker

Tracked locally as markdown files in `.scratch/<feature>/`. See `docs/agents/issue-tracker.md`.

### Triage labels

Standard 5 canonical triage roles (`needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`). See `docs/agents/triage-labels.md`.

### Domain docs

Single-context layout (`GLOSSARY.md` + `docs/adr/` at repo root). See `docs/agents/domain.md`.

## Release workflow

当用户要求“发布版本”或“发布 release”时，执行标准发布流程：
1. 本地测试与校验：在 `web` 目录运行 `pnpm lint && pnpm build`，并确保 `internal/api/dist` 已存在（或执行 `rm -rf internal/api/dist && cp -r web/dist internal/api/dist`），随后运行 `go test ./cmd/... ./internal/...` 确保测试全部通过。
2. 版本号处理：
   - 若用户明确指定了版本号（例如“发布 v0.2.0”），校验其符合 `vX.Y.Z` 规范后直接使用；
   - 若用户未指定版本号，通过 `git tag --sort=-v:refname` 查询最新版本，并检查自上次发布以来的 Git 提交记录按语义化规范（SemVer）自动推导：
     - 若仅包含修复、样式或小优化（如 `fix:`），递增修订号 Patch（如 `v0.1.0` -> `v0.1.1`）；
     - 若包含新特性、新功能（如 `feat:`），递增次版本号 Minor（如 `v0.1.0` -> `v0.2.0`）；
     - 若包含重大架构变更或不兼容变动，递增主版本号 Major（如 `v0.1.0` -> `v1.0.0`）。
   确保代码已提交并推送到 `main` 分支。
3. 创建带附注的 Git Tag 并推送到远程：`git tag -a vX.Y.Z -m "Release vX.Y.Z"` 配合 `git push origin vX.Y.Z`。
4. 推送 Tag 会自动触发 `.github/workflows/release.yml`，自动构建多架构镜像并同步发布至 Docker Hub (`kougami132/tspeek`) 和 GHCR (`ghcr.io/kougami132/tspeek`)。

