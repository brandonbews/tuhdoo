#!/usr/bin/env bash
# End-to-end smoke test for the npm distribution, run locally (not CI):
# cross-compiles the four binaries, assembles the packages with prepare.js,
# installs the main + current-platform tarballs into a fresh git repo, and
# checks the acceptance behaviors: init works, status reports local-only,
# the launcher adds nothing to the binary's output, the MCP shim serves the
# eleven verbs through it, and SIGTERM on the launcher reaches the binary.
#
#   npm/smoke.sh            # uses a temp dir, cleans up after itself
set -euo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
# Short base dir on purpose: the daemon's unix socket lives under the test
# repo's .git/tuhdoo, and macOS caps socket paths at 103 bytes — the default
# mktemp location (/var/folders/...) blows that limit.
work="$(mktemp -d /tmp/tuhdoo-smoke.XXXXXX)"
daemon_pid=""
cleanup() {
  if [ -n "$daemon_pid" ] && kill -0 "$daemon_pid" 2>/dev/null; then
    kill -TERM "$daemon_pid" 2>/dev/null || true
    for _ in $(seq 1 50); do kill -0 "$daemon_pid" 2>/dev/null || break; sleep 0.1; done
  fi
  rm -rf "$work"
}
trap cleanup EXIT

version="0.0.0-smoke.$$"
platform="$(node -p process.platform)"
cpu="$(node -p process.arch)"
case "$cpu" in
  x64) goarch=amd64 ;;
  arm64) goarch=arm64 ;;
  *) echo "unsupported test arch: $cpu" >&2; exit 1 ;;
esac

echo "== cross-compile four targets"
for target in darwin/arm64 darwin/amd64 linux/arm64 linux/amd64; do
  os="${target%/*}"; arch="${target#*/}"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
    go -C "$repo" build -ldflags "-X main.version=v${version}" \
    -o "$work/dist/${os}_${arch}/tuhdoo" ./cmd/tuhdoo
done

echo "== assemble packages"
node "$repo/npm/prepare.js" "$version" "$work/dist" "$work/out"

echo "== pack main + ${platform}-${cpu}"
(cd "$work" && npm pack --silent "./out/tuhdoo" "./out/${platform}-${cpu}")

echo "== install into a fresh repo"
mkdir "$work/testrepo"
cd "$work/testrepo"
git init -q
git config user.email "smoke@example.com"
git config user.name "Smoke Test"
npm init -y >/dev/null
# Both tarballs in one install: the platform tarball satisfies the main
# package's optionalDependency, so nothing is fetched from the registry.
npm i -D --silent "$work/tuhdoo-${version}.tgz" "$work/tuhdoo-${platform}-${cpu}-${version}.tgz"

echo "== launcher output is byte-identical to the binary's"
direct="$("$work/dist/${platform}_${goarch}/tuhdoo" version)"
via_npx="$(npx tuhdoo version)"
[ "$direct" = "$via_npx" ] || { echo "FAIL: output differs: '$direct' vs '$via_npx'" >&2; exit 1; }
[ "$via_npx" = "tuhdoo v${version}" ] || { echo "FAIL: unexpected version output: '$via_npx'" >&2; exit 1; }

echo "== npx tuhdoo init + status"
npx tuhdoo init >/dev/null
status_out="$(npx tuhdoo status)"
daemon_pid="$(node -p 'JSON.parse(require("fs").readFileSync(".git/tuhdoo/daemon.json","utf8")).pid' 2>/dev/null || true)"
case "$status_out" in
  *local-only*) ;;
  *) echo "FAIL: status does not report local-only:" >&2; echo "$status_out" >&2; exit 1 ;;
esac

echo "== MCP shim serves the eleven verbs through the launcher"
mcp_out="$(
  {
    printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"smoke","version":"0"}}}'
    printf '%s\n' '{"jsonrpc":"2.0","method":"notifications/initialized"}'
    printf '%s\n' '{"jsonrpc":"2.0","id":2,"method":"tools/list"}'
    sleep 2
  } | ./node_modules/.bin/tuhdoo mcp
)"
tool_count="$(printf '%s\n' "$mcp_out" | node -e '
  let n = -1;
  require("readline").createInterface({ input: process.stdin }).on("line", (l) => {
    if (!l.trim()) return;
    const m = JSON.parse(l); // any non-JSON line on stdout = shim garbled it
    if (m.id === 2) n = m.result.tools.length;
  }).on("close", () => { console.log(n); });
')"
[ "$tool_count" = "11" ] || { echo "FAIL: expected 11 tools, got $tool_count" >&2; exit 1; }

echo "== SIGTERM on the launcher reaches the binary"
# stdin must stay open — on EOF the MCP shim exits by design. $! of a
# backgrounded pipeline is the last command's pid, i.e. the launcher's.
sleep 60 | ./node_modules/.bin/tuhdoo mcp >/dev/null 2>&1 &
shim_pid=$!
sleep 1
child_pid="$(pgrep -P "$shim_pid" || true)"
[ -n "$child_pid" ] || { echo "FAIL: no child binary under launcher" >&2; exit 1; }
kill -TERM "$shim_pid"
for _ in $(seq 1 50); do
  kill -0 "$child_pid" 2>/dev/null || break
  sleep 0.1
done
if kill -0 "$child_pid" 2>/dev/null; then
  echo "FAIL: binary survived SIGTERM to the launcher" >&2
  kill -KILL "$child_pid" 2>/dev/null || true
  exit 1
fi
wait "$shim_pid" 2>/dev/null && shim_status=0 || shim_status=$?
[ "$shim_status" -gt 128 ] || echo "note: launcher exited $shim_status (expected signal death >128)"

echo "PASS: npm distribution smoke test"
