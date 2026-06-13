#!/usr/bin/env bash
# Adversarial containment suite for the gVisor stdio sandbox (US4 / SC-002, SC-003).
#
# Runs hostile payloads inside a container launched with the SAME flags the
# gateway's ContainerRuntime uses, and asserts each attempt is contained. Run on
# a Docker host that has the runsc (gVisor) runtime registered — e.g. inside the
# Lima VM provisioned by .claude/skills/dev-setup/scripts/sandbox-up.sh:
#   limactl shell mcp -- sudo bash tests/security/adversarial.sh
set -u

RUNTIME="${SANDBOX_RUNTIME:-runsc}"
IMAGE="${SANDBOX_IMAGE:-alpine}"
# The production untrusted-execution flags (research R1/R7; ContainerRuntime).
FLAGS=(--rm "--runtime=${RUNTIME}" --network none --cap-drop ALL
  --security-opt no-new-privileges --read-only --tmpfs /tmp
  --pids-limit 128 --memory 256m)

pass=0
fail=0
ok()   { printf "  \033[32mPASS\033[0m  %s\n" "$1"; pass=$((pass + 1)); }
bad()  { printf "  \033[31mFAIL\033[0m  %s  (got: %s)\n" "$1" "$2"; fail=$((fail + 1)); }

# contained NAME SHELL_CMD : the in-container script must echo CONTAINED on the
# safe path (and anything else on the unsafe path).
contained() {
  local name="$1" cmd="$2"
  local out
  out="$(docker run "${FLAGS[@]}" "$IMAGE" sh -c "$cmd" 2>&1 | tr -d '\r')"
  if printf '%s' "$out" | grep -q CONTAINED; then ok "$name"; else bad "$name" "$out"; fi
}

echo "Adversarial containment suite (runtime=${RUNTIME}, image=${IMAGE})"
echo

contained "cloud metadata egress blocked (169.254.169.254)" \
  'wget -T2 -q -O- http://169.254.169.254/ >/dev/null 2>&1 && echo REACHED || echo CONTAINED'
contained "public internet egress blocked (1.1.1.1)" \
  'wget -T2 -q -O- http://1.1.1.1/ >/dev/null 2>&1 && echo REACHED || echo CONTAINED'
contained "internal RFC-1918 egress blocked (10.0.0.1)" \
  'wget -T2 -q -O- http://10.0.0.1/ >/dev/null 2>&1 && echo REACHED || echo CONTAINED'
contained "isolated guest kernel (gVisor, not host)" \
  'grep -qi gvisor /proc/version && echo CONTAINED || echo HOSTKERNEL'
contained "read-only rootfs enforced" \
  'touch /etc/intruder 2>/dev/null && echo WROTE || echo CONTAINED'
contained "privileged op denied (mount; caps dropped)" \
  'mount -t proc none /mnt 2>/dev/null && echo MOUNTED || echo CONTAINED'
contained "no host filesystem leaked into sandbox" \
  '[ -e /Users ] || [ -e /host ] && echo HOSTFS || echo CONTAINED'

echo
echo "Note: memory (--memory) and process (--pids-limit) caps are enforced by the"
echo "runtime; their flag construction is covered by sandbox.TestContainerRuntime_BuildArgs."
echo
if [ "$fail" -eq 0 ]; then
  echo -e "\033[32mAll $pass containment checks passed.\033[0m"
  exit 0
fi
echo -e "\033[31m${fail} containment check(s) FAILED — HC-3 boundary breach.\033[0m"
exit 1
