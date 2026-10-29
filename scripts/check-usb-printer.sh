#!/usr/bin/env bash
# =========================================================
# SheetHappens - Ubuntu Host USB Printer Diagnostic Script
#
# Inspects connected USB buses, highlights HP (Vendor ID 03f0,
# including the HP DeskJet Ink Advantage 2375) and other USB
# printers, and verifies /dev/bus/usb node permissions.
# =========================================================
set -euo pipefail

BOLD="\033[1m"
GREEN="\033[0;32m"
YELLOW="\033[1;33m"
RED="\033[0;31m"
RESET="\033[0m"

info() { printf "${BOLD}[INFO]${RESET} %s\n" "$*"; }
ok()   { printf "${GREEN}[ OK ]${RESET} %s\n" "$*"; }
warn() { printf "${YELLOW}[WARN]${RESET} %s\n" "$*"; }
err()  { printf "${RED}[ERR ]${RESET} %s\n" "$*"; }

printf "${BOLD}=== SheetHappens USB Printer Diagnostic ===${RESET}\n\n"

# 1. Check lsusb availability
if ! command -v lsusb >/dev/null 2>&1; then
    err "'lsusb' is not installed. Install it with: sudo apt-get update && sudo apt-get install -y usbutils"
    exit 1
fi

info "Enumerating all USB devices via lsusb..."
lsusb
printf "\n"

# 2. Check specifically for HP Vendor ID (03f0)
info "Scanning for HP devices (Vendor ID 03f0 — e.g. HP DeskJet Ink Advantage 2375)..."
HP_MATCHES="$(lsusb -d 03f0: || true)"
if [[ -n "${HP_MATCHES}" ]]; then
    ok "Detected HP USB hardware:"
    printf "  %s\n" "${HP_MATCHES}"
else
    warn "No HP USB device (Vendor ID 03f0) detected. Check the USB cable and printer power."
fi
printf "\n"

# 3. Check for any USB Printer Class (bInterfaceClass 07) across all vendors
info "Scanning sysfs for USB Printer class interfaces (Class 07)..."
FOUND_CLASS07=0
for iface in /sys/bus/usb/devices/*/bInterfaceClass; do
    [[ -f "${iface}" ]] || continue
    if [[ "$(cat "${iface}" 2>/dev/null || true)" == "07" ]]; then
        FOUND_CLASS07=1
        DEV_DIR="$(dirname "${iface}")/.."
        PRODUCT="$(cat "${DEV_DIR}/product" 2>/dev/null || echo "Unknown Product")"
        MANUFACTURER="$(cat "${DEV_DIR}/manufacturer" 2>/dev/null || echo "Unknown Vendor")"
        VID="$(cat "${DEV_DIR}/idVendor" 2>/dev/null || echo "????")"
        PID="$(cat "${DEV_DIR}/idProduct" 2>/dev/null || echo "????")"
        ok "USB Printer interface: ${MANUFACTURER} ${PRODUCT} (${VID}:${PID})"
    fi
done
if [[ "${FOUND_CLASS07}" -eq 0 ]]; then
    warn "No active USB Printer Class (07) interfaces reported in /sys/bus/usb/devices."
fi
printf "\n"

# 4. Inspect /dev/bus/usb permissions
info "Inspecting /dev/bus/usb character devices and permissions..."
if [[ ! -d /dev/bus/usb ]]; then
    err "/dev/bus/usb does not exist on this host."
    exit 1
fi

ls -la /dev/bus/usb
printf "\n"

info "Device nodes (group lp / root access check):"
find /dev/bus/usb -type c -exec ls -l {} +
printf "\n"

# 5. If running Docker, check whether the sheethappens-cups container sees the USB bus
if command -v docker >/dev/null 2>&1 && docker ps --format '{{.Names}}' | grep -qx 'sheethappens-cups'; then
    info "Checking USB visibility inside 'sheethappens-cups' container..."
    docker exec sheethappens-cups lsusb || warn "Could not run lsusb inside container"
    info "Querying CUPS USB backend (lpinfo -v | grep usb)..."
    USB_URIS="$(docker exec sheethappens-cups lpinfo -v 2>/dev/null | grep -i 'usb://' || true)"
    if [[ -n "${USB_URIS}" ]]; then
        ok "CUPS discovered USB printer URI(s):"
        printf "  %s\n" "${USB_URIS}"
    else
        warn "CUPS did not report a usb:// URI yet."
    fi
fi

printf "\n${GREEN}Diagnostic complete.${RESET}\n"
