#!/bin/sh
# This script may run as root or as a non-root user (e.g. when the image is
# started with `USER crow` or with `runAsNonRoot: true` in Kubernetes).
#
# When running as root, it sets up log rotation (cron + logrotate) and then
# switches to the specified user via su-exec.
#
# When running as a non-root user, the cron-based log rotation setup is
# skipped (it requires root to write /etc/periodic and start crond) and the
# binary is exec'd directly without su-exec.
set -e

# Get target user from first argument (empty or missing = stay as current user)
if [ $# -gt 0 ]; then
  TARGET_USER="$1"
  shift  # Remove user argument, leaving only the command
else
  TARGET_USER=""
fi

# Detect whether we are running as root (UID 0). This determines whether the
# logrotate setup and the su-exec user switch can be performed.
if [ "$(id -u)" = "0" ]; then
  IS_ROOT=1
else
  IS_ROOT=0
fi

# Default configuration with WOODPECKER_* fallback for backward compatibility
CROW_LOGROTATE_ENABLED="${CROW_LOGROTATE_ENABLED:-${WOODPECKER_LOGROTATE_ENABLED:-auto}}"
CROW_LOGROTATE_SCHEDULE="${CROW_LOGROTATE_SCHEDULE:-${WOODPECKER_LOGROTATE_SCHEDULE:-0 0 * * *}}"
CROW_LOGROTATE_RETAIN_DAYS="${CROW_LOGROTATE_RETAIN_DAYS:-${WOODPECKER_LOGROTATE_RETAIN_DAYS:-7}}"

# Get effective log file value (respecting WOODPECKER_LOG_FILE fallback)
LOG_FILE="${CROW_LOG_FILE:-${WOODPECKER_LOG_FILE:-stderr}}"

# Validate retain days is a positive integer
if ! echo "$CROW_LOGROTATE_RETAIN_DAYS" | grep -qE '^[0-9]+$' || [ "$CROW_LOGROTATE_RETAIN_DAYS" -lt 1 ]; then
  echo "Warning: CROW_LOGROTATE_RETAIN_DAYS must be a positive integer (got: $CROW_LOGROTATE_RETAIN_DAYS), using default: 7"
  CROW_LOGROTATE_RETAIN_DAYS=7
fi

# Determine if we should enable log rotation
should_enable_rotation() {
  # Explicit true
  [ "$CROW_LOGROTATE_ENABLED" = "true" ] && return 0

  # Explicit false
  [ "$CROW_LOGROTATE_ENABLED" = "false" ] && return 1

  # Auto mode: enable if LOG_FILE points to actual file (not stdout/stderr)
  [ -n "$LOG_FILE" ] && \
  [ "$LOG_FILE" != "stdout" ] && \
  [ "$LOG_FILE" != "stderr" ]
}

# Set up log rotation. The cron-based setup requires root (it writes to
# /etc/periodic, /var/spool/cron and starts crond). When running as a
# non-root user, skip the setup and emit a warning if the user explicitly
# requested file-based rotation.
if should_enable_rotation && [ "$IS_ROOT" = "0" ]; then
  echo "Warning: log rotation requested (CROW_LOG_FILE=$LOG_FILE) but the container is not running as root."
  echo "         Internal cron-based log rotation requires root. Skipping setup."
  echo "         For non-root containers, route logs to stdout/stderr and rotate them outside the container,"
  echo "         or override the user (e.g. runAsUser: 0 in Kubernetes) if you need the built-in rotation."
elif should_enable_rotation; then
  echo "Setting up log rotation for $LOG_FILE (schedule: $CROW_LOGROTATE_SCHEDULE, retain: $CROW_LOGROTATE_RETAIN_DAYS days)"

  # Ensure log directory exists (may require root in some configurations)
  LOG_DIR="$(dirname "$LOG_FILE")"
  if [ ! -d "$LOG_DIR" ]; then
    echo "Creating log directory: $LOG_DIR"
    mkdir -p "$LOG_DIR" 2>/dev/null || echo "Warning: Could not create $LOG_DIR (may need root)"
  fi

  # Create logrotate config in /tmp (works with read-only root filesystem)
  # Note: Using actual LOG_FILE path, not hardcoded pattern
  # No 'su' directive - we already run as the appropriate user
  cat > /tmp/crow-logrotate.conf <<EOF
$LOG_FILE {
    daily
    rotate $CROW_LOGROTATE_RETAIN_DAYS
    compress
    delaycompress
    missingok
    notifempty
    copytruncate
}
EOF

  # Create logrotate wrapper script that logs to stdout/stderr for container visibility
  cat > /tmp/run-logrotate.sh <<'EOF'
#!/bin/sh
echo "[logrotate] Starting log rotation at $(date)"
/usr/sbin/logrotate -s /tmp/logrotate.status /tmp/crow-logrotate.conf 2>&1 | sed 's/^/[logrotate] /'
echo "[logrotate] Completed"
EOF
  chmod +x /tmp/run-logrotate.sh

  # Determine which user should run the cron job
  CRON_USER="${TARGET_USER:-root}"

  # Use Alpine's /etc/periodic/ structure for cron jobs
  # Default schedule is daily (0 0 * * *), so use /etc/periodic/daily
  # For custom schedules, we'll add to root's crontab
  PERIODIC_DIR=""
  case "$CROW_LOGROTATE_SCHEDULE" in
    "0 0 * * *") PERIODIC_DIR="/etc/periodic/daily" ;;
    "0 * * * *") PERIODIC_DIR="/etc/periodic/hourly" ;;
    "*/15 * * * *") PERIODIC_DIR="/etc/periodic/15min" ;;
    *) # Custom schedule - use root crontab
      # BusyBox crond requires TABs between fields, not spaces
      mkdir -p /var/spool/cron/crontabs
      SCHEDULE_WITH_TABS=$(echo "$CROW_LOGROTATE_SCHEDULE" | tr ' ' '\t')
      if [ "$CRON_USER" = "root" ]; then
        printf "%s\t/tmp/run-logrotate.sh\n" "$SCHEDULE_WITH_TABS" >> /var/spool/cron/crontabs/root
      else
        printf "%s\tsu-exec %s /tmp/run-logrotate.sh\n" "$SCHEDULE_WITH_TABS" "$CRON_USER" >> /var/spool/cron/crontabs/root
      fi
      ;;
  esac

  # If using periodic directory, create wrapper script there
  if [ -n "$PERIODIC_DIR" ]; then
    mkdir -p "$PERIODIC_DIR"
    cat > "$PERIODIC_DIR/crow-logrotate" <<'EOFSCRIPT'
#!/bin/sh
TARGET_USER="$1"
if [ -n "$TARGET_USER" ]; then
  su-exec "$TARGET_USER" /tmp/run-logrotate.sh
else
  /tmp/run-logrotate.sh
fi
EOFSCRIPT
    # Replace placeholder with actual user
    sed -i "s/TARGET_USER=\"\$1\"/TARGET_USER=\"$CRON_USER\"/" "$PERIODIC_DIR/crow-logrotate"
    chmod +x "$PERIODIC_DIR/crow-logrotate"
  fi

  # Start crond in background mode (daemon) - output goes to syslog
  # Note: Using -b (background) instead of -f (foreground) because with -f and &,
  # crond becomes orphaned when we exec and turns into a zombie
  crond -b -l 2
  echo "Log rotation enabled (crond started in background, job runs as: $CRON_USER)"
else
  echo "Log rotation disabled (LOG_FILE not configured or set to stdout/stderr)"
fi

# Execute crow binary (replaces this process). su-exec requires root, so only
# attempt the user switch when we are root and a target user was supplied.
# When we are already running as a non-root user the binary is exec'd
# directly, regardless of what TARGET_USER says.
if [ "$IS_ROOT" = "1" ] && [ -n "$TARGET_USER" ]; then
  exec su-exec "$TARGET_USER" "$@"
else
  exec "$@"
fi
