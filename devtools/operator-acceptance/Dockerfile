# Isolated acceptance host only. No Go, browser, enrollment or test-PKI tooling.
FROM --platform=linux/amd64 ubuntu:24.04
RUN printf '#!/bin/sh\nexit 101\n' > /usr/sbin/policy-rc.d && chmod 755 /usr/sbin/policy-rc.d \
 && apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends nginx systemd systemd-sysv python3-venv ca-certificates curl procps iproute2 openssh-server \
 && python3 -m venv /opt/certbot && /opt/certbot/bin/pip install --no-cache-dir 'certbot==5.8.0' \
 && ln -s /opt/certbot/bin/certbot /usr/bin/certbot \
 && rm -rf /var/lib/apt/lists/*
CMD ["sleep", "infinity"]
