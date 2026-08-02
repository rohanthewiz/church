# Builds any church site binary (ccswm, cema) from the Go workspace.
#
# The build context must be the PARENT directory holding church/, ccswm/,
# cema/, and go.work — the sites depend on the church module by workspace
# resolution, not by published version, so all three modules have to be
# in the context:
#
#   cd ~/projs/go/church
#   DOCKER_BUILDKIT=1 docker build -f church/deploy/docker/Dockerfile \
#     --build-arg SITE=ccswm \
#     -t ghcr.io/rohanthewiz/ccswm:$(git -C ccswm rev-parse --short HEAD) .
#
# BuildKit is REQUIRED, not just preferred: the exclusions that keep this
# context from being ~2.9 GB live in Dockerfile.dockerignore next to this
# file, and only BuildKit honors the per-Dockerfile ignore convention. The
# classic builder would silently ship church_mobile/ (2.6 GB), cema/sermons/,
# node_modules/, and — the real problem — each site's cfg/options.yml with
# live Stripe and SMTP credentials into the build cache. deploy.sh sets
# DOCKER_BUILDKIT=1 for exactly this reason.
#
# One ARG selects the site; everything else is identical, so both images
# stay structurally in sync by construction.
ARG SITE=ccswm

# go1.26 matches the toolchain directive in go.work.
#
# CGO IS REQUIRED. resource/chimage (inline article-image resizing) imports
# github.com/h2non/bimg, which is a cgo binding to libvips — with
# CGO_ENABLED=0 the package does not compile at all ("undefined: Options",
# "undefined: Gravity", …), so this was never a size/portability trade-off.
# The rest of the tree genuinely is pure Go (lib/pq, bytdb embedded), which
# is why the earlier CGO_ENABLED=0 looked plausible right up until it was
# built. Consequence: the binary links libvips dynamically, so the runtime
# stage must install matching vips libraries (below) and cannot be scratch
# or distroless.
FROM golang:1.26-alpine AS build
ARG SITE
# build-base: gcc + musl-dev for cgo. vips-dev: libvips headers + .so to link.
RUN apk add --no-cache build-base vips-dev
WORKDIR /src
COPY go.work go.work.sum* ./
COPY church/ church/
COPY ccswm/ ccswm/
COPY cema/ cema/
# cema/ carries its own nested go.work (use . + ../church). `go build -C cema`
# resolves the innermost workspace first, so leaving it in place would build
# the cema image against a *different* workspace than the ccswm image — and
# drag in a gitignored cema/go.work.sum whose checksums can fail verification.
# Removing it forces both sites through the one workspace at /src/go.work.
RUN rm -f cema/go.work cema/go.work.sum
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 go build -C ${SITE} -trimpath -ldflags='-s -w' -o /out/site .

# Runtime: the binary, its libvips dependency chain, CA certs (Stripe, S3,
# SMTP-over-TLS all need them), and tzdata (event times are pastor-local).
FROM alpine:3.20
ARG SITE
RUN apk add --no-cache ca-certificates tzdata vips \
 && addgroup -g 1000 app && adduser -D -u 1000 -G app app
WORKDIR /app
COPY --from=build /out/site /app/${SITE}
# cfg/ is NOT baked in — options.yml AND random_seeds.txt both arrive from
# the <site>-config Secret mounted over /app/cfg at runtime. random_seeds.txt
# is not optional: resource/auth's init() log.Fatal()s when it can't open
# cfg/random_seeds.txt, which happens before main() and takes the process
# with it. An image without that file cannot start under any configuration.
#
# dist/ (built CSS/JS) ships with the site repo, so it rides along here.
# dist/img/ is deliberately left as an empty mount point: it holds
# admin-uploaded article images, which the Deployment backs with a
# persistent volume — anything baked in at that path would be shadowed.
COPY --from=build /src/${SITE}/dist/ /app/dist/
RUN mkdir -p /app/cfg /app/log /app/dist/img && chown -R app:app /app
USER app
# Shell-form indirection because ENTRYPOINT exec-form can't expand ARG;
# exec replaces the shell so signals still reach the binary (pid 1).
ENV SITE_BIN=/app/${SITE}
ENTRYPOINT exec "${SITE_BIN}"
