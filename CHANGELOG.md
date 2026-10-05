# Changelog

## [Unreleased]

## [0.1.1] - 2026-10-05

### Added

- Conditional item visibility via `show_when` field (outer OR, inner AND condition groups, with cascading)
- JSON Schema generation from Go types (`alma schema [document|record]`, `just generate_schema`)
- CI step to validate generated schemas are up-to-date (`just schema-check`)
- (UI) Requirements section split into Hard/Soft groups for readability
- Lightweight formatting in design option descriptions (inline \`code\` and line breaks)
- `just release` target for safe, branch-guarded release preparation

## [0.1.0] - 2026-10-04

### Added

- Declarative YAML/JSON document format for structured design decisions
- Interactive web UI with live constraint evaluation
- Decision capture with rationale, attendees, and append-only history
- Confirmation modal when state changes would invalidate the current decision
- Multi-format export (YAML, JSON, Markdown)
- Print / PDF support with print-optimized styles
- Light and dark theme with system preference detection
- Keyboard support: Escape closes export dropdown and confirmation modal
- Screen reader announcements for decision changes (`aria-live`)
- Mobile browser chrome color matching via `theme-color` meta
- CLI commands: `validate`, `render`, `serve`, `export`
- Cross-platform binaries (linux/darwin, amd64/arm64)
- CI/CD: PR validation, automated releases on tag push
- Dependabot for Go, npm, and GitHub Actions dependencies
- CodeQL security scanning
- Playwright end-to-end UI tests
- Accessibility testing with pa11y

[Unreleased]: https://github.com/inf0-dev/alma/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/inf0-dev/alma/releases/tag/v0.1.1
[0.1.0]: https://github.com/inf0-dev/alma/releases/tag/v0.1.0
