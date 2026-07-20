# Builds any church site binary (ccswm, cema) from the Go workspace.
#
# The build context must be the PARENT directory holding church/, ccswm/,
# cema/, and go.work — the sites depend on the church module by workspace
# resolution, not by published version, so all three modules have to be
# in the context:
#
#   cd ~/projs/go/church
#   docker build -f church/deploy/docker/Dockerfile --build-arg SITE=ccswm \
#     -t ghcr.io/rohanthewiz/ccswm:$(git -C ccswm rev-parse --short HEAD) .
#
# One ARG selects the site; everything else is identical, so both images
# stay structurally in sync by construction.
ARG SITE=ccswm

# go1.26 matches the toolchain directive in go.work; alpine keeps the
# builder small and the final binary is static anyway (lib/pq is pure Go,
# bytdb is embedded — CGO not needed).
FROM golang:1.26-alpine AS build
ARG SITE
WORKDIR /src
COPY go.work go.work.sum* ./
COPY church/ church/
COPY ccswm/ ccswm/
COPY cema/ cema/
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -C ${SITE} -trimpath -ldflags='-s -w' -o /out/site .

# Distroless-style final stage: just the binary, CA certs (Stripe, S3,
# SMTP-over-TLS all need them), and tzdata (event times are pastor-local).
FROM alpine:3.20
ARG SITE
RUN apk add --no-cache ca-certificates tzdata \
 && addgroup -g 1000 app && adduser -D -u 1000 -G app app
WORKDIR /app
COPY --from=build /out/site /app/${SITE}
# cfg/options.yml is NOT baked in — it mounts from a k8s Secret at runtime.
# dist/ (built CSS/JS) ships with the site repo, so it rides along here.
COPY --from=build /src/${SITE}/dist/ /app/dist/
RUN mkdir -p /app/cfg /app/log && chown -R app:app /app
USER app
# Shell-form indirection because ENTRYPOINT exec-form can't expand ARG;
# exec replaces the shell so signals still reach the binary (pid 1).
ENV SITE_BIN=/app/${SITE}
ENTRYPOINT exec "${SITE_BIN}"
