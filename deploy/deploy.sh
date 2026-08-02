#!/usr/bin/env bash
#
# deploy.sh — one-command deploy of the church sites to a Linode/Akamai LKE
# cluster. Every phase is idempotent: re-running is the normal way to apply a
# change, not a recovery action.
#
#   ./deploy/deploy.sh all              # full install, in dependency order
#   ./deploy/deploy.sh images sites     # rebuild + redeploy after a code change
#   ./deploy/deploy.sh verify           # health check without changing anything
#   SITES=ccswm ./deploy/deploy.sh all  # one site only
#
# Phase order is not cosmetic — each step's inputs are produced by the one
# before it:
#
#   preflight → infra → base → seeds → secrets → images → sites → verify
#                 │       │                                  │
#                 │       └ ClusterIssuer needs cert-manager's CRDs
#                 └ NodeBalancer IP must exist before DNS can point at it,
#                   and DNS must resolve before an Ingress is applied, or
#                   the HTTP-01 challenge fails and Let's Encrypt starts
#                   counting failures against the rate limit.
#
# Written for bash 3.2 (what macOS ships): no associative arrays, no mapfile,
# no ${var,,}. Site metadata is read out of the manifests rather than
# duplicated here, so the YAML stays the single source of truth for domains.

set -euo pipefail

# --------------------------------------------------------------------------
# Configuration (all overridable from the environment)
# --------------------------------------------------------------------------

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHURCH_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"    # the church framework repo
WORKSPACE_DIR="$(cd "$CHURCH_DIR/.." && pwd)" # holds church/ ccswm/ cema/ go.work
K8S_DIR="$SCRIPT_DIR/k8s"

# Space-separated. Adding a site here plus a sites/<name>.yaml is the whole
# onboarding path.
SITES="${SITES:-ccswm cema}"

NAMESPACE="${NAMESPACE:-churches}"
REGISTRY="${REGISTRY:-ghcr.io/rohanthewiz}"

# Empty means "whatever the chart repo serves today". Pin both for
# reproducible clusters once you know the versions you want to live with.
INGRESS_NGINX_VERSION="${INGRESS_NGINX_VERSION:-}"
CERT_MANAGER_VERSION="${CERT_MANAGER_VERSION:-}"

# Object-storage credentials for backups + WAL shipping. Not committed;
# see backup.env.sample. A per-site file wins over the shared one.
BACKUP_ENV_FILE="${BACKUP_ENV_FILE:-$SCRIPT_DIR/backup.env}"

ASSUME_YES="${ASSUME_YES:-false}"
ROLLOUT_TIMEOUT="${ROLLOUT_TIMEOUT:-300s}"
CERT_TIMEOUT="${CERT_TIMEOUT:-600s}"

# --------------------------------------------------------------------------
# Output helpers
# --------------------------------------------------------------------------

if [ -t 1 ]; then
	C_RESET=$'\033[0m'; C_BOLD=$'\033[1m'; C_RED=$'\033[31m'
	C_GREEN=$'\033[32m'; C_YELLOW=$'\033[33m'; C_BLUE=$'\033[34m'
else
	C_RESET=""; C_BOLD=""; C_RED=""; C_GREEN=""; C_YELLOW=""; C_BLUE=""
fi

phase() { printf '\n%s══ %s ══%s\n' "$C_BOLD$C_BLUE" "$*" "$C_RESET"; }
info()  { printf '  %s\n' "$*"; }
ok()    { printf '  %s✓%s %s\n' "$C_GREEN" "$C_RESET" "$*"; }
warn()  { printf '  %s!%s %s\n' "$C_YELLOW" "$C_RESET" "$*" >&2; }
die()   { printf '\n%serror:%s %s\n' "$C_RED$C_BOLD" "$C_RESET" "$*" >&2; exit 1; }

confirm() {
	[ "$ASSUME_YES" = "true" ] && return 0
	printf '  %s?%s %s [y/N] ' "$C_YELLOW" "$C_RESET" "$1"
	read -r reply </dev/tty || return 1
	case "$reply" in [yY]*) return 0 ;; *) return 1 ;; esac
}

need_cmd() {
	command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

# --------------------------------------------------------------------------
# Site metadata — parsed from the manifests so nothing is declared twice
# --------------------------------------------------------------------------

site_manifest() { printf '%s/sites/%s.yaml\n' "$K8S_DIR" "$1"; }

# Every `- host: x` under the Ingress rules. Also what the DNS precheck and
# the post-deploy smoke test iterate over.
site_domains() {
	sed -n 's/^[[:space:]]*-[[:space:]]*host:[[:space:]]*\([^[:space:]]*\).*/\1/p' \
		"$(site_manifest "$1")"
}

site_source_dir() { printf '%s/%s\n' "$WORKSPACE_DIR" "$1"; }

# Tag images with the site repo's short SHA. A moving :latest tag would make
# "which build is actually running" unanswerable, and with imagePullPolicy
# defaulting on tag name it also makes rollbacks unreliable.
site_image() {
	local site="$1" sha
	sha="$(git -C "$(site_source_dir "$site")" rev-parse --short HEAD 2>/dev/null || echo dev)"
	if ! git -C "$(site_source_dir "$site")" diff --quiet HEAD 2>/dev/null; then
		sha="${sha}-dirty"
	fi
	printf '%s/%s:%s\n' "$REGISTRY" "$site" "$sha"
}

kc() { kubectl --namespace "$NAMESPACE" "$@"; }

# --------------------------------------------------------------------------
# preflight — fail here, not halfway through a cluster mutation
# --------------------------------------------------------------------------

cmd_preflight() {
	phase "Preflight"

	need_cmd kubectl; need_cmd helm; need_cmd git; need_cmd openssl; need_cmd sed
	ok "tooling present"

	kubectl cluster-info >/dev/null 2>&1 || die "no reachable cluster — check your kubeconfig context"
	local ctx; ctx="$(kubectl config current-context)"
	ok "cluster reachable via context: ${C_BOLD}${ctx}${C_RESET}"
	confirm "Deploy to this context?" || die "aborted at context confirmation"

	for site in $SITES; do
		[ -f "$(site_manifest "$site")" ] || die "missing manifest: $(site_manifest "$site")"
		[ -d "$(site_source_dir "$site")" ] || die "missing site source dir: $(site_source_dir "$site")"
		local domains; domains="$(site_domains "$site" | tr '\n' ' ')"
		[ -n "$(echo "$domains" | tr -d ' ')" ] || die "no Ingress hosts found in $(site_manifest "$site")"
		ok "$site → $domains"
	done

	[ -f "$WORKSPACE_DIR/go.work" ] || die "expected a Go workspace at $WORKSPACE_DIR/go.work (docker build context)"
	ok "build context: $WORKSPACE_DIR"
}

# --------------------------------------------------------------------------
# infra — ingress controller and cert-manager
# --------------------------------------------------------------------------

cmd_infra() {
	phase "Cluster infrastructure"

	helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx >/dev/null 2>&1 || true
	helm repo add jetstack https://charts.jetstack.io >/dev/null 2>&1 || true
	helm repo update >/dev/null
	ok "helm repos updated"

	local ingressArgs=""
	[ -n "$INGRESS_NGINX_VERSION" ] && ingressArgs="--version $INGRESS_NGINX_VERSION"
	# --install makes this both the create and the upgrade path.
	# externalTrafficPolicy=Local preserves the client IP through the
	# NodeBalancer, which the app's request logging and any future rate
	# limiting both want.
	# shellcheck disable=SC2086
	helm upgrade --install ingress-nginx ingress-nginx/ingress-nginx \
		--namespace ingress-nginx --create-namespace \
		--set controller.service.externalTrafficPolicy=Local \
		--wait --timeout 10m $ingressArgs
	ok "ingress-nginx installed"

	local certArgs=""
	[ -n "$CERT_MANAGER_VERSION" ] && certArgs="--version $CERT_MANAGER_VERSION"
	# shellcheck disable=SC2086
	helm upgrade --install cert-manager jetstack/cert-manager \
		--namespace cert-manager --create-namespace \
		--set crds.enabled=true \
		--wait --timeout 10m $certArgs
	ok "cert-manager installed"

	local ip; ip="$(ingress_ip)"
	if [ -n "$ip" ]; then
		ok "NodeBalancer external IP: ${C_BOLD}${ip}${C_RESET}"
	else
		warn "NodeBalancer has no external IP yet — Linode takes a minute to provision it"
	fi
}

ingress_ip() {
	kubectl -n ingress-nginx get svc ingress-nginx-controller \
		-o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || true
}

# --------------------------------------------------------------------------
# base — namespace and ClusterIssuer (needs cert-manager CRDs to exist)
# --------------------------------------------------------------------------

cmd_base() {
	phase "Base objects"
	kubectl apply -f "$K8S_DIR/00-namespace.yaml"
	kubectl apply -f "$K8S_DIR/01-clusterissuer.yaml"
	ok "namespace + ClusterIssuer applied"
}

# --------------------------------------------------------------------------
# seeds — cfg/random_seeds.txt, without which the process cannot start
# --------------------------------------------------------------------------

# resource/auth's init() opens cfg/random_seeds.txt and log.Fatal()s when it
# can't — before main(), so a site without this file is an unconditional crash
# loop. Generating one is safe even for a long-running site: the pool is an
# entropy source for NEW salts and tokens (each user's salt is stored next to
# their hash in the DB), so replacing it never invalidates existing logins.
# Existing files are never touched — the sites' own seeds stay theirs.
cmd_seeds() {
	phase "Crypto seed files"
	for site in $SITES; do
		local seedFile="$(site_source_dir "$site")/cfg/random_seeds.txt"
		if [ -f "$seedFile" ]; then
			ok "$site: cfg/random_seeds.txt present ($(wc -l <"$seedFile" | tr -d ' ') lines)"
			continue
		fi
		info "$site: generating cfg/random_seeds.txt (72 × 19-char)"
		mkdir -p "$(dirname "$seedFile")"
		: >"$seedFile"
		local i=0
		while [ "$i" -lt 72 ]; do
			openssl rand -base64 24 | tr -d '/+=' | cut -c1-19 >>"$seedFile"
			i=$((i + 1))
		done
		chmod 600 "$seedFile"
		ok "$site: generated (gitignored — back it up with your other site secrets)"
	done
}

# --------------------------------------------------------------------------
# secrets — <site>-config and <site>-backup
# --------------------------------------------------------------------------

cmd_secrets() {
	phase "Secrets"

	[ -f "$BACKUP_ENV_FILE" ] || die "missing $BACKUP_ENV_FILE — copy backup.env.sample and fill it in"

	for site in $SITES; do
		local srcDir; srcDir="$(site_source_dir "$site")"
		local optionsFile="$srcDir/cfg/options.yml"
		local seedFile="$srcDir/cfg/random_seeds.txt"

		[ -f "$optionsFile" ] || die "$site: missing cfg/options.yml (copy cfg/options-sample.yml and fill it in)"
		[ -f "$seedFile" ] || die "$site: missing cfg/random_seeds.txt — run './deploy/deploy.sh seeds' first"

		# The pod runs with APP_ENV=production, and config.getOptionsForEnvironment
		# log.Fatal()s on a missing section. Catching that here beats catching it
		# in a CrashLoopBackOff.
		grep -q '^production:' "$optionsFile" \
			|| die "$site: cfg/options.yml has no 'production:' section (the pod sets APP_ENV=production)"

		# server.port / use_tls are deliberately NOT checked: the Deployment
		# overrides both via SERVER_PORT and USE_TLS precisely so the file can
		# keep its bare-metal values.
		kc create secret generic "${site}-config" \
			--from-file=options.yml="$optionsFile" \
			--from-file=random_seeds.txt="$seedFile" \
			--dry-run=client -o yaml | kc apply -f - >/dev/null
		ok "$site-config (options.yml + random_seeds.txt)"

		# Per-site backup credentials override the shared file, so one site can
		# live in a different region or bucket without forking the whole setup.
		local envFile="$BACKUP_ENV_FILE"
		[ -f "$SCRIPT_DIR/backup.$site.env" ] && envFile="$SCRIPT_DIR/backup.$site.env"

		# shellcheck disable=SC1090
		set -a; . "$envFile"; set +a

		: "${OBJ_ENDPOINT:?OBJ_ENDPOINT not set in $envFile}"
		: "${OBJ_BUCKET:?OBJ_BUCKET not set in $envFile}"
		: "${OBJ_ACCESS_KEY:?OBJ_ACCESS_KEY not set in $envFile}"
		: "${OBJ_SECRET_KEY:?OBJ_SECRET_KEY not set in $envFile}"
		# Both S3 clients (resource/dbbackup and db/replicate) default a blank
		# region to us-east-1. On a bucket in any other Linode region that
		# breaks SigV4 signing, and the failure is only ever logged — backups
		# and WAL shipping stop while the site keeps serving normally. So it is
		# required here rather than defaulted.
		# No apostrophe in this message on purpose: bash parses the word part of
		# ${var:?word} with quote awareness, so a lone ' opens a quoted section
		# and swallows the rest of the file.
		: "${OBJ_REGION:?OBJ_REGION not set in $envFile — must match the region the bucket lives in}"

		# Reuse an already-deployed token rather than minting a new one on every
		# run: rotating it silently would break the backup CronJob's next call.
		local token
		token="$(kc get secret "${site}-backup" -o jsonpath='{.data.BACKUP_TOKEN}' 2>/dev/null || true)"
		if [ -n "$token" ]; then
			token="$(printf '%s' "$token" | openssl base64 -d -A)"
			info "$site: reusing existing BACKUP_TOKEN"
		else
			token="$(openssl rand -hex 32)"
			info "$site: minted a new BACKUP_TOKEN"
		fi

		kc create secret generic "${site}-backup" \
			--from-literal=OBJ_ENDPOINT="$OBJ_ENDPOINT" \
			--from-literal=OBJ_REGION="$OBJ_REGION" \
			--from-literal=OBJ_BUCKET="$OBJ_BUCKET" \
			--from-literal=OBJ_ACCESS_KEY="$OBJ_ACCESS_KEY" \
			--from-literal=OBJ_SECRET_KEY="$OBJ_SECRET_KEY" \
			--from-literal=BACKUP_TOKEN="$token" \
			--dry-run=client -o yaml | kc apply -f - >/dev/null
		ok "$site-backup (endpoint=$OBJ_ENDPOINT region=$OBJ_REGION bucket=$OBJ_BUCKET)"

		unset OBJ_ENDPOINT OBJ_REGION OBJ_BUCKET OBJ_ACCESS_KEY OBJ_SECRET_KEY
	done
}

# --------------------------------------------------------------------------
# images — build and push
# --------------------------------------------------------------------------

cmd_images() {
	phase "Images"

	need_cmd docker
	docker info >/dev/null 2>&1 || die "docker daemon not reachable — start Docker Desktop"

	# BuildKit is mandatory, not a preference: the exclusions that keep the
	# context from being ~2.9 GB — and that keep each site's live options.yml
	# out of the build cache — live in Dockerfile.dockerignore, and only
	# BuildKit honors the per-Dockerfile ignore file.
	export DOCKER_BUILDKIT=1

	for site in $SITES; do
		local image; image="$(site_image "$site")"
		case "$image" in
			*-dirty) warn "$site: working tree is dirty — tagging $image" ;;
		esac

		# dist/ is built by the site's npm toolchain and copied into the image
		# as-is; an empty dist/css means the site would serve unstyled pages.
		local distDir; distDir="$(site_source_dir "$site")/dist/css"
		if [ ! -d "$distDir" ] || [ -z "$(ls -A "$distDir" 2>/dev/null)" ]; then
			warn "$site: dist/css is empty — run 'npm run build-css' in $(site_source_dir "$site")"
			confirm "Build $site anyway?" || die "aborted"
		fi

		info "$site: building $image"
		docker build \
			-f "$CHURCH_DIR/deploy/docker/Dockerfile" \
			--build-arg SITE="$site" \
			-t "$image" \
			"$WORKSPACE_DIR"
		docker push "$image"
		ok "$site: pushed $image"
	done
}

# --------------------------------------------------------------------------
# sites — DNS precheck, then apply
# --------------------------------------------------------------------------

# Applying an Ingress before DNS resolves means cert-manager fires an HTTP-01
# challenge that cannot succeed. Let's Encrypt rate-limits failed
# authorizations (5/hour/account/hostname), so a few impatient retries can
# lock out issuance for the rest of the hour. Hence a check, not a hope.
check_dns() {
	local expectedIP="$1" site="$2" allResolved=0
	command -v dig >/dev/null 2>&1 || { warn "dig not found — skipping DNS precheck"; return 0; }
	for domain in $(site_domains "$site"); do
		local actual; actual="$(dig +short A "$domain" | tail -1)"
		if [ -z "$actual" ]; then
			warn "$domain does not resolve"; allResolved=1
		elif [ "$actual" != "$expectedIP" ]; then
			warn "$domain → $actual (expected $expectedIP)"; allResolved=1
		else
			ok "$domain → $actual"
		fi
	done
	return $allResolved
}

cmd_sites() {
	phase "Sites"

	local ip; ip="$(ingress_ip)"
	[ -n "$ip" ] || die "ingress-nginx has no external IP — run the 'infra' phase and wait for the NodeBalancer"
	info "NodeBalancer IP: $ip"

	for site in $SITES; do
		if ! check_dns "$ip" "$site"; then
			warn "$site: DNS is not (yet) pointing at the NodeBalancer."
			warn "TLS issuance will fail and burn Let's Encrypt rate limit until it does."
			confirm "Apply $site anyway?" || { info "$site: skipped"; continue; }
		fi

		local image; image="$(site_image "$site")"
		# Substitute the pinned tag over the manifest's :latest placeholder at
		# apply time. Keeps the committed YAML readable and keeps the running
		# spec pinned to an exact build — without a templating dependency.
		sed "s|image: .*/${site}:latest|image: ${image}|" "$(site_manifest "$site")" \
			| kubectl apply -f -
		ok "$site: applied at $image"
	done

	for site in $SITES; do
		info "$site: waiting for rollout"
		kc rollout status "deploy/$site" --timeout="$ROLLOUT_TIMEOUT" \
			|| die "$site: rollout failed — 'kubectl -n $NAMESPACE logs deploy/$site' has the reason"
		ok "$site: rolled out"
	done
}

# --------------------------------------------------------------------------
# verify — read-only health check
# --------------------------------------------------------------------------

cmd_verify() {
	phase "Verify"

	local failed=0
	for site in $SITES; do
		local pod
		pod="$(kc get pod -l "app=$site" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
		if [ -z "$pod" ]; then
			warn "$site: no pod found"; failed=1; continue
		fi
		local ready
		ready="$(kc get pod "$pod" -o jsonpath='{.status.containerStatuses[0].ready}' 2>/dev/null || echo false)"
		[ "$ready" = "true" ] && ok "$site: pod $pod ready" || { warn "$site: pod $pod NOT ready"; failed=1; }

		# Certificate readiness is separate from pod readiness — the site can
		# serve fine over plain HTTP while ACME is still failing.
		if kc get certificate "${site}-tls" >/dev/null 2>&1; then
			if kc wait --for=condition=Ready "certificate/${site}-tls" --timeout="$CERT_TIMEOUT" >/dev/null 2>&1; then
				ok "$site: TLS certificate ready"
			else
				warn "$site: TLS certificate NOT ready — 'kubectl -n $NAMESPACE describe certificate ${site}-tls'"
				failed=1
			fi
		else
			warn "$site: no Certificate resource yet"; failed=1
		fi

		# In-cluster smoke test, so it works before DNS has propagated.
		if kc exec "$pod" -- wget -qO- --timeout=5 "http://localhost:4000/healthz" 2>/dev/null | grep -q ok; then
			ok "$site: /healthz responding"
		else
			warn "$site: /healthz did not respond"; failed=1
		fi

		# Replication lag is the one signal that says the DB is genuinely
		# protected. A 503 here means shipping is off or misconfigured.
		local token
		token="$(kc get secret "${site}-backup" -o jsonpath='{.data.BACKUP_TOKEN}' 2>/dev/null | openssl base64 -d -A 2>/dev/null || true)"
		if [ -n "$token" ]; then
			local repl
			repl="$(kc exec "$pod" -- wget -qO- --timeout=5 \
				--header="Authorization: Bearer $token" \
				"http://localhost:4000/api/admin/db/replication" 2>/dev/null || true)"
			if [ -n "$repl" ]; then
				ok "$site: replication $repl"
			else
				warn "$site: replication status unavailable (off, or not yet shipping)"
			fi
		fi

		for domain in $(site_domains "$site"); do
			info "$site: https://$domain"
		done
	done

	[ "$failed" -eq 0 ] && ok "all checks passed" || warn "some checks failed (see above)"
	return 0
}

# --------------------------------------------------------------------------
# Dispatch
# --------------------------------------------------------------------------

usage() {
	cat <<EOF
Usage: ${0##*/} [--yes] <phase>...

Phases:
  preflight   Verify tooling, cluster context, manifests, build context
  infra       helm install ingress-nginx + cert-manager (creates NodeBalancer)
  base        Apply namespace + Let's Encrypt ClusterIssuer
  seeds       Generate cfg/random_seeds.txt where missing (never overwrites)
  secrets     Create/update <site>-config and <site>-backup
  images      docker build + push one image per site
  sites       DNS precheck, apply site manifests, wait for rollout
  verify      Read-only health check (pods, certs, /healthz, replication)
  all         Every phase above, in order

Environment:
  SITES="ccswm cema"          Sites to act on
  NAMESPACE=churches          Target namespace
  REGISTRY=ghcr.io/rohanthewiz
  BACKUP_ENV_FILE=deploy/backup.env
  INGRESS_NGINX_VERSION=, CERT_MANAGER_VERSION=   Pin chart versions
  ASSUME_YES=true             Non-interactive (same as --yes)

Between DNS and TLS: point every domain's A record at the NodeBalancer IP
printed by 'infra' BEFORE running 'sites', or ACME issuance will fail.
EOF
}

main() {
	local phases=""
	while [ $# -gt 0 ]; do
		case "$1" in
			--yes|-y) ASSUME_YES=true ;;
			-h|--help) usage; exit 0 ;;
			preflight|infra|base|seeds|secrets|images|sites|verify) phases="$phases $1" ;;
			all) phases="$phases preflight infra base seeds secrets images sites verify" ;;
			*) usage; die "unknown argument: $1" ;;
		esac
		shift
	done

	[ -n "$(echo "$phases" | tr -d ' ')" ] || { usage; exit 1; }

	# preflight is cheap and catches the mistakes that are expensive later, so
	# it runs for any phase set that doesn't already include it.
	case " $phases " in *" preflight "*) ;; *) cmd_preflight ;; esac

	for p in $phases; do "cmd_$p"; done

	printf '\n%sDone.%s\n' "$C_GREEN$C_BOLD" "$C_RESET"
}

main "$@"
