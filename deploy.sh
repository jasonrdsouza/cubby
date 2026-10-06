#!/usr/bin/env bash
# Install or upgrade cubby on a systemd host from a GitHub release.
#
# Usage:
#   ./deploy.sh              install the latest release
#   ./deploy.sh v1.6         install a specific release
#   ./deploy.sh --rollback   restore the previously installed binary
#
# Environment overrides:
#   CUBBY_BIN         install path              (default /usr/local/bin/cubby)
#   CUBBY_SERVICE     systemd unit name         (default cubby)
#   CUBBY_HEALTH_URL  URL checked after restart (default http://localhost:8383/)
#   CUBBY_DB          if set, this DB file is backed up (with the service
#                     stopped) to $CUBBY_DB.bak before the new binary starts
set -euo pipefail

REPO=jasonrdsouza/cubby
BIN=${CUBBY_BIN:-/usr/local/bin/cubby}
SERVICE=${CUBBY_SERVICE:-cubby}
HEALTH_URL=${CUBBY_HEALTH_URL:-http://localhost:8383/}
DB=${CUBBY_DB:-}

SUDO=sudo
[ "$(id -u)" -eq 0 ] && SUDO=

log() { echo "==> $*"; }
die() { echo "error: $*" >&2; exit 1; }

healthy() {
  for _ in $(seq 1 10); do
    curl -fs -o /dev/null "$HEALTH_URL" && return 0
    sleep 1
  done
  return 1
}

restart() {
  if [ -n "$DB" ]; then
    log "Stopping $SERVICE to back up $DB"
    $SUDO systemctl stop "$SERVICE"
    $SUDO cp -p "$DB" "$DB.bak"
    $SUDO systemctl start "$SERVICE"
  else
    $SUDO systemctl restart "$SERVICE"
  fi
}

if [ "${1:-}" = "--rollback" ]; then
  [ -f "$BIN.prev" ] || die "no previous binary at $BIN.prev"
  log "Rolling back to $BIN.prev"
  $SUDO install -m 0755 "$BIN.prev" "$BIN"
  $SUDO systemctl restart "$SERVICE"
  healthy || die "$SERVICE is not responding at $HEALTH_URL after rollback"
  log "Rolled back"
  exit 0
fi

case "$(uname -m)" in
  x86_64 | amd64) ARCH=amd64 ;;
  aarch64 | arm64) ARCH=arm64 ;;
  *) die "unsupported architecture: $(uname -m)" ;;
esac

VERSION=${1:-}
if [ -z "$VERSION" ]; then
  # /releases/latest redirects to /releases/tag/<version>
  VERSION=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest")
  VERSION=${VERSION##*/}
fi
ASSET=cubby-linux-$ARCH
BASE_URL=https://github.com/$REPO/releases/download/$VERSION

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

log "Downloading $ASSET $VERSION"
curl -fsSL -o "$TMP/$ASSET" "$BASE_URL/$ASSET" ||
  die "could not download $ASSET for $VERSION (does that release exist?)"
curl -fsSL -o "$TMP/SHA256SUMS" "$BASE_URL/SHA256SUMS" ||
  die "$VERSION has no SHA256SUMS (releases before v1.6 are unsupported; use --rollback or install manually)"

log "Verifying checksum"
(cd "$TMP" && grep " $ASSET\$" SHA256SUMS | sha256sum -c --quiet -) ||
  die "checksum verification failed for $ASSET"

if [ -f "$BIN" ]; then
  $SUDO cp -p "$BIN" "$BIN.prev"
fi
$SUDO install -m 0755 "$TMP/$ASSET" "$BIN"

log "Restarting $SERVICE"
restart
if ! healthy; then
  echo "error: $SERVICE is not responding at $HEALTH_URL" >&2
  if [ -f "$BIN.prev" ]; then
    log "Rolling back to the previous binary"
    $SUDO install -m 0755 "$BIN.prev" "$BIN"
    $SUDO systemctl restart "$SERVICE"
    healthy || die "$SERVICE is still not responding after rollback; check: journalctl -u $SERVICE"
    log "Rolled back; $SERVICE is up on the previous binary"
  fi
  exit 1
fi
log "Deployed cubby $VERSION"
