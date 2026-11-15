## What and why

<!-- One topic per PR. Link the issue: Fixes #123 -->

## Tested on

- [ ] Model / hardware:
- [ ] Harness only (`just harness <model>`)

## Checklist

- [ ] `just check` passes
- [ ] One fragment `.changes/<slug>.md` (or label `no-changelog`)
- [ ] Contributor credits checked: original authors/co-authors preserved; fragment acknowledgements and shared credits updated where applicable
- [ ] Texts added in every language (en, de, es, fr, it, nl, zh)
- [ ] No edit to `plugin/ugreen-nas-control.plg`, `CHANGELOG.md` or any generated file (`just generated-check`)
- [ ] Docs updated where behaviour changed (`just docs` if `web/models/` changed)
- [ ] PR title is `type(scope): summary`
