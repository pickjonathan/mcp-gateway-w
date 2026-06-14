# Base image that stdio MCP servers run in, under gVisor (runsc) or Kata for
# isolation. The server command (e.g. `npx -y <pkg>` or `uvx <pkg>`) is supplied
# as the container command by the sandbox ContainerRuntime.
FROM node:20-bookworm-slim

RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      python3 python3-venv pipx ca-certificates curl unzip groff less \
 && rm -rf /var/lib/apt/lists/*

# --- 004 isolation proof: pre-bake the AWS MCP server + AWS CLI ---------------
# The egress-restricted gVisor sandbox (--network mcp-sandbox-egress, internal)
# cannot reach PyPI, so the AWS MCP server MUST be baked in (no uvx@latest fetch
# at launch). The AWS CLI v2 honors AWS_ENDPOINT_URL / AWS_ENDPOINT_URL_S3, which
# is how the server is pointed at the local ministack emulator.
RUN set -eux; \
    arch="$(dpkg --print-architecture)"; \
    case "$arch" in amd64) a=x86_64 ;; arm64) a=aarch64 ;; *) a="$arch" ;; esac; \
    curl -fsSL "https://awscli.amazonaws.com/awscli-exe-linux-${a}.zip" -o /tmp/awscli.zip; \
    unzip -q /tmp/awscli.zip -d /tmp; \
    /tmp/aws/install; \
    rm -rf /tmp/aws /tmp/awscli.zip; \
    aws --version

# Install the AWS MCP server globally (console script on PATH). The server
# registration's `command` is `awslabs.aws-api-mcp-server` (see
# specs/004-aws-mcp-isolation-proof/contracts/tenant-aws-setup.md).
ENV PIPX_HOME=/opt/pipx \
    PIPX_BIN_DIR=/usr/local/bin
RUN pipx install awslabs.aws-api-mcp-server \
 && ls -1 /usr/local/bin | grep -i aws

# node/npm/npx come from the base image; pipx provides isolated Python tools.
# The runtime mounts the rootfs read-only (writable /tmp only) and drops all caps.
USER node
ENTRYPOINT []
