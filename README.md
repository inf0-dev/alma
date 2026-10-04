<p align="center">
  <img src="docs/assets/alma-sm.png" alt="alma" width="200">
</p>

# alma

**AL**ignment **MA**trix. A declarative tool for structured design decisions.

Define requirements, items, and design options in a YAML/JSON document. alma evaluates which options are viable based on your constraints and renders an interactive UI for alignment sessions.

<p align="center">
  <img src="docs/assets/demo-upload.webp" alt="Upload a document and explore options" width="720">
</p>

<p align="center">
  <img src="docs/assets/demo-decision.webp" alt="Make a decision with rationale and attendees" width="720">
</p>

<p align="center">
  <img src="docs/assets/demo-theme.webp" alt="Toggle between light and dark mode" width="720">
</p>

## Install

Download a pre-built binary from the [latest release](https://github.com/inf0-dev/alma/releases/latest), or build from source:

```bash
just build    # outputs to ./_output/alma
```

## Quick Start

```bash
# start the server with the upload UI
alma serve

# start with a document pre-loaded
alma serve -p docs/examples/database-selection.yaml

# use a custom port
just serve 3000
```

Open `http://localhost:8080`, upload a document, and start aligning.

See [`docs/examples/`](docs/examples/) for sample documents.

## CLI

```
alma validate -p <file>           Validate a document or record
alma render -p <file> [-o file]   Render to self-contained HTML
alma export -p <file> [-o fmt]    Export a record (yaml, json, md)
alma serve [-p <file>] [-a addr]  Start the web server
```

## Features

- **Interactive evaluation.** Toggle requirements, answer items, and watch option statuses update in real time.
- **Decision capture.** Pick an option, record who was present, add rationale, and finalize.
- **Append-only history.** Every decision, undo, and change is recorded as an immutable audit entry.
- **Multi-format export.** Download as YAML, JSON, or Markdown from the UI or CLI.
- **PDF output.** Print-optimized layout via the browser's native print dialog.
- **Dark mode.** System-aware with manual toggle.
- **Fully static HTML.** `alma render` produces a single self-contained file that works offline.

## Architecture

```
Document (YAML/JSON)
    │
    ▼
  Engine ──▶ evaluates constraints, effects, blocks
    │
    ▼
  Record ──▶ requirements, items, options, decision, history
    │
    ├──▶ Renderer ──▶ self-contained HTML
    └──▶ Server   ──▶ interactive UI with live sync
```

1. **Document** defines requirements (hard/soft), items (choices, text, numbers), and design options with constraints, effects, and blocks.
2. **Engine** evaluates which options are possible, blocked, or eliminated based on current answers and checked requirements.
3. **Renderer** produces a self-contained HTML page with all styles and scripts inlined.
4. **Server** serves the interactive UI, syncs state on every change, and handles export and finalization.

## Development

Requires [Go](https://go.dev/) and [Just](https://github.com/casey/just).

```bash
just test                  # run all tests with merged coverage
just go_test_unit          # unit tests only
just go_test_integration   # integration tests only
just update_golden         # regenerate golden files
just lint                  # golangci-lint
just fmt                   # format Go, CSS, and JS
just fmt-check             # verify formatting (CI)
just demo                  # render demo HTML and open in browser
just a11y                  # accessibility checks with pa11y
```

## License

[MIT](LICENSE)
