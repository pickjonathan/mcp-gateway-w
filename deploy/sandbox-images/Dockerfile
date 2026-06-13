# Base image that stdio MCP servers run in, under gVisor (runsc) or Kata for
# isolation. The server command (e.g. `npx -y <pkg>` or `uvx <pkg>`) is supplied
# as the container command by the sandbox ContainerRuntime.
FROM node:20-bookworm-slim

RUN apt-get update \
 && apt-get install -y --no-install-recommends python3 python3-venv pipx ca-certificates \
 && rm -rf /var/lib/apt/lists/*

# node/npm/npx come from the base image; pipx provides isolated Python tools.
# The runtime mounts the rootfs read-only (writable /tmp only) and drops all caps.
USER node
ENTRYPOINT []
