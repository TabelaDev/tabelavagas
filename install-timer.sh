#!/usr/bin/env bash
set -euo pipefail

# install-timer.sh — instala systemd user timer pra rodar tabelavagas all
# todo dia às 21:00. Persistent=true garante catch-up se a máquina estiver
# desligada. --only-new evita re-notificar vagas já avisadas.

JOB_NAME="tabelavagas"
TIMER_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user"
SERVICE_FILE="$TIMER_DIR/${JOB_NAME}.service"
TIMER_FILE="$TIMER_DIR/${JOB_NAME}.timer"
LOG_DIR="$HOME/.local/state/${JOB_NAME}"
LOG_FILE="$LOG_DIR/${JOB_NAME}.log"

# Check if tabelavagas is installed
if ! command -v tabelavagas &>/dev/null; then
    echo "erro: tabelavagas não encontrado no PATH" >&2
    echo "instale com: go install github.com/TabelaDev/tabelavagas@latest" >&2
    exit 1
fi

# Create log directory
mkdir -p "$LOG_DIR"

# Create systemd user directory
mkdir -p "$TIMER_DIR"

# Write service unit
cat > "$SERVICE_FILE" <<EOF
[Unit]
Description=tabelavagas daily collect + rank + notify

[Service]
Type=oneshot
ExecStart=$(which tabelavagas) all --only-new
StandardOutput=append:${LOG_FILE}
StandardError=append:${LOG_FILE}
EOF

# Write timer unit
cat > "$TIMER_FILE" <<EOF
[Unit]
Description=tabelavagas daily (21:00)

[Timer]
OnCalendar=*-*-* 21:00:00
Persistent=true

[Install]
WantedBy=timers.target
EOF

# Reload and enable
systemctl --user daemon-reload
systemctl --user enable --now "${JOB_NAME}.timer"

echo "timer instalado:"
echo "  ${SERVICE_FILE}"
echo "  ${TIMER_FILE}"
echo ""
echo "próximo run: $(systemctl --user list-timers "${JOB_NAME}.timer" --no-pager | tail -1)"
echo "logs: ${LOG_FILE}"
echo ""
echo "comandos úteis:"
echo "  systemctl --user status ${JOB_NAME}.timer    # ver status"
echo "  systemctl --user start ${JOB_NAME}.service   # rodar agora"
echo "  journalctl --user -u ${JOB_NAME}             # ver logs"
