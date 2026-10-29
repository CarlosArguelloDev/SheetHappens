#!/bin/sh
# =========================================================
# SheetHappens - CUPS container entrypoint
#
#  1. Seeds /etc/cups from image defaults if the volume is empty.
#  2. Installs the managed cupsd.conf (remote admin on 0.0.0.0:631).
#  3. Provisions the CUPS admin user (member of `lpadmin`).
#  4. Execs cupsd in the foreground as PID 1.
# =========================================================
set -eu

log() { printf '[sheethappens-cups] %s\n' "$*"; }
die() { printf '[sheethappens-cups] ERROR: %s\n' "$*" >&2; exit 1; }

CUPS_CONF_DIR=/etc/cups
DEFAULTS_DIR=/opt/cups-defaults
MANAGED_CONF=/opt/sheethappens/cupsd.conf

# ---------------------------------------------------------------------------
# 1. Seed configuration directory (first run with an empty volume/bind mount)
# ---------------------------------------------------------------------------
if [ ! -f "${CUPS_CONF_DIR}/cups-files.conf" ]; then
    log "Empty ${CUPS_CONF_DIR} detected, seeding from image defaults"
    cp -a "${DEFAULTS_DIR}/." "${CUPS_CONF_DIR}/"
fi
mkdir -p "${CUPS_CONF_DIR}/ppd" "${CUPS_CONF_DIR}/ssl" /run/cups /var/spool/cups/tmp

# ---------------------------------------------------------------------------
# 2. Managed cupsd.conf
# ---------------------------------------------------------------------------
install -m 0640 -o root -g lp "${MANAGED_CONF}" "${CUPS_CONF_DIR}/cupsd.conf"
log "Installed managed cupsd.conf (Listen 0.0.0.0:631)"

# ---------------------------------------------------------------------------
# 3. Admin user provisioning
# ---------------------------------------------------------------------------
CUPS_ADMIN_USER="${CUPS_ADMIN_USER:-}"
CUPS_ADMIN_PASSWORD="${CUPS_ADMIN_PASSWORD:-}"

if [ -z "${CUPS_ADMIN_USER}" ] || [ -z "${CUPS_ADMIN_PASSWORD}" ]; then
    die "CUPS_ADMIN_USER and CUPS_ADMIN_PASSWORD must be set (see .env.example)"
fi

# POSIX-portable username: lowercase letter/underscore first, max 32 chars.
case "${CUPS_ADMIN_USER}" in
    root|lp|daemon|nobody)
        die "CUPS_ADMIN_USER '${CUPS_ADMIN_USER}' is a reserved system account" ;;
esac
if ! printf '%s' "${CUPS_ADMIN_USER}" | grep -Eq '^[a-z_][a-z0-9_-]{0,31}$'; then
    die "CUPS_ADMIN_USER must match ^[a-z_][a-z0-9_-]{0,31}\$"
fi

if [ "${#CUPS_ADMIN_PASSWORD}" -lt 8 ]; then
    die "CUPS_ADMIN_PASSWORD must be at least 8 characters long"
fi

if id "${CUPS_ADMIN_USER}" >/dev/null 2>&1; then
    log "Admin user '${CUPS_ADMIN_USER}' already exists, refreshing credentials"
else
    log "Creating admin user '${CUPS_ADMIN_USER}'"
    useradd --system --no-create-home --shell /usr/sbin/nologin "${CUPS_ADMIN_USER}"
fi
usermod -aG lpadmin "${CUPS_ADMIN_USER}"

# Password is fed via stdin (never as a CLI argument visible in `ps`).
printf '%s:%s\n' "${CUPS_ADMIN_USER}" "${CUPS_ADMIN_PASSWORD}" | chpasswd

# Do not leak the secret into cupsd or its filter/backend child processes.
unset CUPS_ADMIN_PASSWORD

# ---------------------------------------------------------------------------
# 4. Start the scheduler
# ---------------------------------------------------------------------------
# Validate the configuration before handing over PID 1.
cupsd -t -c "${CUPS_CONF_DIR}/cupsd.conf" >/dev/null || die "Invalid cupsd.conf"

log "Starting cupsd (web admin: https://<HOST-IP>:631/admin)"
exec /usr/sbin/cupsd -f
