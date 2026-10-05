set unstable

out_dir := absolute_path("./_output")
cov_dir := out_dir / "coverage"
version := shell("git describe --tags --always --dirty")
ldflags := "-s -w -X main.version=" + version

# initialize directory if it does not exist
init_dir_ine(dir) := shell(f"mkdir -p {{ dir }}")

# run Go unit tests with coverage
go_test_unit:
    {{ init_dir_ine(cov_dir) }}
    @ go test -tags unit ./... -coverprofile={{ cov_dir }}/unit.out

# run Go integration tests with coverage
go_test_integration:
    {{ init_dir_ine(cov_dir) }}
    @ go test -tags integration ./... -coverprofile={{ cov_dir }}/integration.out

# run e2e ui tests (playwright)
[arg("headless", pattern="(true|false)", help="whether to run in headless mode (no browser)")]
e2e_ui headless='true':
    @ cd e2e/ui && npx playwright test {{ if headless == "true" { "" } else { "--ui" } }}

# run all e2e tests
e2e: e2e_ui

# run all tests and merge coverage
test: go_test_unit go_test_integration test_coverage

# merge unit + integration coverage into a single report
test_coverage:
    @ go tool gocovmerge {{ cov_dir }}/unit.out {{ cov_dir }}/integration.out > {{ cov_dir }}/merged.out
    @ echo "--- merged coverage ---"
    @ go tool cover -func={{ cov_dir }}/merged.out | grep total

lint:
    @ golangci-lint run --timeout 5m

fmt: fmt-go fmt-frontend

# format Go source files
fmt-go:
    @ go fmt ./...

# format CSS and JS files with Prettier (HTML templates skipped; template syntax breaks Prettier)
fmt-frontend:
    @ npx --yes prettier --write "internal/**/*.{css,js}" --log-level warn

# check if formatting is correct
fmt-check: fmt
    @ git diff --exit-code

# run accessibility check on rendered demo (light + dark)
a11y: demo
    @ echo "--- light mode ---"
    @ pa11y {{ out_dir }}/demo.html || true
    @ sed 's/<html lang="en">/<html lang="en" data-theme="dark">/' {{ out_dir }}/demo.html > {{ out_dir }}/demo-dark.html
    @ echo "--- dark mode ---"
    @ pa11y {{ out_dir }}/demo-dark.html || true

# render demo HTML and open in browser
demo:
    {{ init_dir_ine(out_dir) }}
    @ go run ./app/cli render -p internal/pkg/renderer/testdata/demo.yaml -o {{ out_dir }}/demo.html
    @ open {{ out_dir }}/demo.html

# generate JSON schemas from Go types
generate_schema:
    @ go run ./internal/cmd/genschema

# check that generated schemas are up-to-date
schema-check: generate_schema
    @ git diff --exit-code docs/schema/

# update golden files for integration tests
update_golden:
    @ go test -tags=integration ./internal/pkg/exporter/ -update
    @ go test -tags=integration ./internal/pkg/renderer/ -update

# run the server
[arg("port", pattern="[0-9]+", help="local port to listen on")]
serve port='8080':
    @ go run ./app/cli serve --address ":{{ port }}"

# check that required dev tools are installed
check-dev:
    #!/usr/bin/env sh
    missing=""
    for tool in go golangci-lint npx pa11y; do
        if command -v "$tool" >/dev/null 2>&1; then
            printf "  %-20s %s\n" "$tool" "$(command -v $tool)"
        else
            printf "  %-20s MISSING\n" "$tool"
            missing="$missing $tool"
        fi
    done
    if [ -n "$missing" ]; then
        echo ""
        echo "Missing:$missing"
        exit 1
    else
        echo "All dev tools found."
    fi

build:
    @ go build -ldflags="{{ ldflags }}" -o {{ out_dir }}/alma ./app/cli

build-release os='darwin' arch='arm64':
    @ GOOS={{ os }} GOARCH={{ arch }} go build -ldflags="{{ ldflags }}" -o {{ out_dir }}/alma-{{ os }}-{{ arch }} ./app/cli

# prepare a release: update CHANGELOG, commit, and print next steps
[arg("tag", pattern="v[0-9]+\\.[0-9]+\\.[0-9]+", help="semver tag to release (e.g. v0.1.0)")]
release tag:
    #!/usr/bin/env bash
    set -eu
    branch=$(git branch --show-current)
    if [ "$branch" = "main" ]; then
        echo "error: do not release from $branch. create a release branch first"
        exit 1
    fi
    ver="{{ tag }}"
    semver="${ver#v}"
    today=$(date +%Y-%m-%d)
    grep -q "## \[Unreleased\]" CHANGELOG.md || { echo "error: no [Unreleased] section in CHANGELOG.md"; exit 1; }
    sed -i "" "s/## \[Unreleased\]/## [$semver] - $today/" CHANGELOG.md
    sed -i "" "s|^\[Unreleased\]:.*|[$semver]: https://github.com/inf0-dev/alma/releases/tag/$ver|" CHANGELOG.md
    awk -v s="$semver" -v v="$ver" '
      /^## \[/{if(index($0,s)){print "## [Unreleased]\n"; print; next}}
      /^\[/{if(index($0,s)){print "[Unreleased]: https://github.com/inf0-dev/alma/compare/" v "...HEAD"; print; next}}
      1' CHANGELOG.md > CHANGELOG.tmp && mv CHANGELOG.tmp CHANGELOG.md
    git add CHANGELOG.md
    git commit -m "release: $ver"
    echo ""
    echo "CHANGELOG updated and committed."
    echo "Next steps:"
    echo "  1. git push && open PR to main"
    echo "  2. After merge, tag the merge commit:"
    echo "     git tag $ver <merge-commit-sha>"
    echo "     git push origin $ver"
