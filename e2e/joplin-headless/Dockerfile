# Headless Joplin Data API container.
#
# Runs the Joplin terminal CLI ("joplin server start") and exposes the
# Data API on port 8080 via socat, since the CLI hard-codes its bind
# address to 127.0.0.1:41184.
#
# Inspired by github.com/jspiers/headless-joplin, rebuilt from scratch
# on current base images with version pinning and security best practices.

FROM node:22.14-alpine3.21

# Pin the Joplin CLI version for reproducible builds.
# Update this when upgrading Joplin.
ARG JOPLIN_VERSION=3.5.1

# Install runtime dependencies:
#   socat  – forwards 0.0.0.0:8080 → 127.0.0.1:41184
#   curl   – used by the healthcheck
#   tini   – proper PID 1 / signal handling
RUN apk add --no-cache \
        socat=1.8.0.1-r0 \
        curl=8.12.1-r0 \
        tini=0.19.0-r3

# Run as non-root for security.
RUN addgroup -S joplin && adduser -S joplin -G joplin
USER joplin
WORKDIR /home/joplin

# Install Joplin CLI at the pinned version.
RUN npm install --no-optional --ignore-scripts joplin@${JOPLIN_VERSION}
ENV PATH="/home/joplin/node_modules/.bin:${PATH}"

# Copy the entrypoint script.
COPY --chown=joplin:joplin entrypoint.sh /home/joplin/entrypoint.sh
RUN chmod +x /home/joplin/entrypoint.sh

EXPOSE 8080

HEALTHCHECK --interval=5s --timeout=3s --start-period=30s --retries=10 \
    CMD curl -sf http://localhost:8080/ping || exit 1

ENTRYPOINT ["tini", "--"]
CMD ["/home/joplin/entrypoint.sh"]
