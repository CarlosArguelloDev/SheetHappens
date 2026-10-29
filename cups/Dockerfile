# syntax=docker/dockerfile:1
# =========================================================
# SheetHappens - Multi-brand CUPS print server
#
# Primary target : HP DeskJet Ink Advantage 2375 (hpcups / HPLIP)
# Also supports  : Gutenprint (Epson, Canon, ...), Brother (brlaser),
#                  ZjStream/foo2zjs lasers, and the OpenPrinting /
#                  Foomatic PPD collections for generic printers.
# =========================================================
FROM debian:bookworm-slim

LABEL org.opencontainers.image.title="sheethappens-cups" \
      org.opencontainers.image.description="Multi-brand CUPS server for SheetHappens" \
      org.opencontainers.image.source="https://github.com/CarlosArguelloDev/SheetHappens"

ENV DEBIAN_FRONTEND=noninteractive \
    LANG=C.UTF-8

# --- Core CUPS stack + vendor drivers --------------------------------------
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
        # CUPS core
        cups \
        cups-client \
        cups-bsd \
        cups-filters \
        # USB diagnostics (lsusb)
        usbutils \
        # HP: DeskJet Ink Advantage 2375 / 2300 series
        hplip \
        printer-driver-hpcups \
        # Multi-vendor driver bundles
        printer-driver-gutenprint \
        printer-driver-brlaser \
        printer-driver-foo2zjs \
        foomatic-db-compressed-ppds \
        openprinting-ppds \
        # Runtime helpers (password provisioning, TLS for the web UI)
        passwd \
        ca-certificates \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/*

# --- Pristine copy of /etc/cups ---------------------------------------------
# /etc/cups is persisted in a Docker volume. If an empty volume/bind mount is
# attached, the entrypoint seeds it from this copy.
RUN cp -a /etc/cups /opt/cups-defaults

# --- SheetHappens configuration ---------------------------------------------
COPY cupsd.conf    /opt/sheethappens/cupsd.conf
COPY entrypoint.sh /usr/local/bin/entrypoint.sh

# Defensive: strip CR characters in case files were checked out with CRLF.
RUN sed -i 's/\r$//' /opt/sheethappens/cupsd.conf /usr/local/bin/entrypoint.sh \
 && chmod 0644 /opt/sheethappens/cupsd.conf \
 && chmod 0755 /usr/local/bin/entrypoint.sh

VOLUME ["/etc/cups"]

EXPOSE 631

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD lpstat -h localhost:631 -r || exit 1

# Provisions the admin user, installs cupsd.conf and execs `cupsd -f`.
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
