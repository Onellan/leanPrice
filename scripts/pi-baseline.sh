#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
target=${PI_SSH_TARGET:-}
output=${PI_BASELINE_OUTPUT:-$root/docs/evidence/B00/pi-baseline.md}

if [ -z "$target" ]; then
  echo 'BLOCKED: set PI_SSH_TARGET=user@raspberry-pi to collect the required target baseline.' >&2
  exit 2
fi
if ! command -v ssh >/dev/null 2>&1; then
  echo 'BLOCKED: OpenSSH client is unavailable.' >&2
  exit 2
fi
if [ -e "$output" ]; then
  echo "Refusing to overwrite existing evidence: $output" >&2
  exit 2
fi
mkdir -p "$(dirname -- "$output")"
temporary=$(mktemp "$(dirname -- "$output")/.pi-baseline.XXXXXX")
trap 'rm -f "$temporary"' EXIT HUP INT TERM

ssh -o BatchMode=yes -o ConnectTimeout=8 "$target" sh -s >"$temporary" <<'REMOTE'
set -eu
printf '# Raspberry Pi host baseline\n\n'
printf '**Captured (UTC):** %s\n\n' "$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
printf '## Host\n\n'
printf '```text\n'
uname -a
if command -v free >/dev/null 2>&1; then free -b; else echo 'free: unavailable'; fi
if command -v docker >/dev/null 2>&1; then
  docker version --format 'Docker server: {{.Server.Version}}'
  docker info --format 'Architecture: {{.Architecture}}; OS: {{.OperatingSystem}}; cgroup version: {{.CgroupVersion}}' || true
else
  echo 'Docker: unavailable'
fi
printf '\n'
printf 'Root cgroup v2 memory.max: '
if [ -r /sys/fs/cgroup/memory.max ]; then cat /sys/fs/cgroup/memory.max; else echo 'unavailable'; fi
printf 'Root cgroup v2 memory.current: '
if [ -r /sys/fs/cgroup/memory.current ]; then cat /sys/fs/cgroup/memory.current; else echo 'unavailable'; fi
printf 'Root cgroup v2 memory.events:\n'
if [ -r /sys/fs/cgroup/memory.events ]; then cat /sys/fs/cgroup/memory.events; else echo 'unavailable'; fi
printf '\nDisk at /data:\n'
if [ -e /data ] && command -v df >/dev/null 2>&1; then df -B1 /data; else echo '/data or df unavailable'; fi
printf '```\n\n'
printf '> This host sample does not certify the leanPrice container cap or application peak. Verify the exact 440,000,000-byte container limit and application cgroup counters during the controlled ARM64 workload.\n'
REMOTE

chmod 600 "$temporary"
ln "$temporary" "$output"
rm -f "$temporary"
trap - EXIT HUP INT TERM
printf 'Pi host baseline written to %s\n' "$output"
