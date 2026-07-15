#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR
readonly APP_USER="shiftz"
readonly DEPLOY_USER="shiftz-deploy"

if [[ "$EUID" -ne 0 ]]; then
  echo "Run with sudo: sudo DEPLOY_PUBLIC_KEY=... ./bootstrap.sh" >&2
  exit 1
fi

for required in shiftz.service shiftz.env.example shiftz-deploy shiftz-rollback; do
  if [[ ! -f "$SCRIPT_DIR/$required" ]]; then
    echo "Missing $SCRIPT_DIR/$required" >&2
    exit 2
  fi
done

export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y --no-install-recommends ca-certificates curl file openssh-server tzdata

if ! id "$APP_USER" >/dev/null 2>&1; then
  useradd --system --home-dir /opt/shiftz --shell /usr/sbin/nologin "$APP_USER"
fi
if ! id "$DEPLOY_USER" >/dev/null 2>&1; then
  useradd --create-home --shell /bin/bash "$DEPLOY_USER"
fi

install -d -o root -g "$APP_USER" -m 0750 /opt/shiftz
install -d -o root -g root -m 0750 /etc/shiftz

if [[ ! -e /etc/shiftz/shiftz.env ]]; then
  install -o root -g root -m 0600 "$SCRIPT_DIR/shiftz.env.example" /etc/shiftz/shiftz.env
  echo "Created /etc/shiftz/shiftz.env; replace every placeholder before deploying."
else
  echo "Keeping existing /etc/shiftz/shiftz.env unchanged."
fi

install -o root -g root -m 0644 "$SCRIPT_DIR/shiftz.service" /etc/systemd/system/shiftz.service
install -o root -g root -m 0755 "$SCRIPT_DIR/shiftz-deploy" /usr/local/sbin/shiftz-deploy
install -o root -g root -m 0755 "$SCRIPT_DIR/shiftz-rollback" /usr/local/sbin/shiftz-rollback

deploy_home="$(getent passwd "$DEPLOY_USER" | cut -d: -f6)"
chmod 0750 "$deploy_home"
install -d -o "$DEPLOY_USER" -g "$DEPLOY_USER" -m 0700 "$deploy_home/.ssh"
touch "$deploy_home/.ssh/authorized_keys"
chown "$DEPLOY_USER:$DEPLOY_USER" "$deploy_home/.ssh/authorized_keys"
chmod 0600 "$deploy_home/.ssh/authorized_keys"

if [[ -n "${DEPLOY_PUBLIC_KEY:-}" ]]; then
  if ! grep -Fqx "$DEPLOY_PUBLIC_KEY" "$deploy_home/.ssh/authorized_keys"; then
    printf '%s\n' "$DEPLOY_PUBLIC_KEY" >> "$deploy_home/.ssh/authorized_keys"
  fi
elif [[ ! -s "$deploy_home/.ssh/authorized_keys" ]]; then
  echo "DEPLOY_PUBLIC_KEY was empty; add a public key to $deploy_home/.ssh/authorized_keys." >&2
fi

usermod -a -G systemd-journal "$DEPLOY_USER"

systemctl_path="$(command -v systemctl)"
cat > /etc/sudoers.d/shiftz-deploy <<EOF
$DEPLOY_USER ALL=(root) NOPASSWD: /usr/local/sbin/shiftz-deploy, /usr/local/sbin/shiftz-rollback, $systemctl_path status shiftz.service, $systemctl_path status shiftz.service --no-pager, $systemctl_path restart shiftz.service
EOF
chmod 0440 /etc/sudoers.d/shiftz-deploy
visudo -cf /etc/sudoers.d/shiftz-deploy

cat > /etc/ssh/sshd_config.d/99-shiftz-hardening.conf <<'EOF'
PasswordAuthentication no
KbdInteractiveAuthentication no
PermitRootLogin no
PubkeyAuthentication yes
EOF
sshd -t
systemctl reload ssh

if ! swapon --show=NAME --noheadings | grep -q .; then
  if [[ ! -f /swapfile ]]; then
    fallocate -l 1G /swapfile
    chmod 0600 /swapfile
    mkswap /swapfile
  fi
  swapon /swapfile
fi
if ! grep -qE '^/swapfile\s' /etc/fstab; then
  printf '/swapfile none swap sw 0 0\n' >> /etc/fstab
fi
cat > /etc/sysctl.d/99-shiftz.conf <<'EOF'
vm.swappiness=10
EOF
sysctl --system >/dev/null

install -d -m 0755 /etc/systemd/journald.conf.d
cat > /etc/systemd/journald.conf.d/shiftz-limits.conf <<'EOF'
[Journal]
SystemMaxUse=100M
RuntimeMaxUse=50M
MaxRetentionSec=14day
EOF
systemctl restart systemd-journald

systemctl daemon-reload
systemctl enable shiftz.service

echo
echo "Bootstrap complete. Next:"
echo "  1. sudoedit /etc/shiftz/shiftz.env"
echo "  2. verify SSH as $DEPLOY_USER"
echo "  3. configure GitHub production secrets"
echo "  4. run the Deploy GCP workflow"
