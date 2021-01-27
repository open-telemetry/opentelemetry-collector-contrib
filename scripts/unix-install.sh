#!/bin/sh
# shellcheck disable=SC2119
# SC2119 -> Use foo "$@" if function's $1 should mean script's $1.

set -e

# Agent Constants
SERVICE_NAME="stanza"
BINARY_NAME="stanza"
DOWNLOAD_BASE="https://github.com/opentelemetry/opentelemetry-log-collection/releases"
PLUGINS_PACKAGE="stanza-plugins.tar.gz"

# Script Constants
PREREQS="curl hostname printf ps sed uname cut tar"
SCRIPT_NAME="$0"
INDENT_WIDTH='  '
indent=""
REQUIRE_SECRET_KEY="false"

# Colors
num_colors=$(tput colors 2>/dev/null)
if test -n "$num_colors" && test "$num_colors" -ge 8; then
  bold="$(tput bold)"
  underline="$(tput smul)"
  # standout can be bold or reversed colors dependent on terminal
  standout="$(tput smso)"
  reset="$(tput sgr0)"
  bg_black="$(tput setab 0)"
  bg_blue="$(tput setab 4)"
  bg_cyan="$(tput setab 6)"
  bg_green="$(tput setab 2)"
  bg_magenta="$(tput setab 5)"
  bg_red="$(tput setab 1)"
  bg_white="$(tput setab 7)"
  bg_yellow="$(tput setab 3)"
  fg_black="$(tput setaf 0)"
  fg_blue="$(tput setaf 4)"
  fg_cyan="$(tput setaf 6)"
  fg_green="$(tput setaf 2)"
  fg_magenta="$(tput setaf 5)"
  fg_red="$(tput setaf 1)"
  fg_white="$(tput setaf 7)"
  fg_yellow="$(tput setaf 3)"
fi

if [ -z "$reset" ]; then
  sed_ignore=''
else
  sed_ignore="/^[$reset]+$/!"
fi

# Helper Functions
printf() {
  if command -v sed >/dev/null; then
    command printf -- "$@" | sed -E "$sed_ignore s/^/$indent/g"  # Ignore sole reset characters if defined
  else
    # Ignore $* suggestion as this breaks the output
    # shellcheck disable=SC2145
    command printf -- "$indent$@"
  fi
}

increase_indent() { indent="$INDENT_WIDTH$indent" ; }
decrease_indent() { indent="${indent#*$INDENT_WIDTH}" ; }

# Color functions reset only when given an argument
bold() { command printf "$bold$*$(if [ -n "$1" ]; then command printf "$reset"; fi)" ; }
underline() { command printf "$underline$*$(if [ -n "$1" ]; then command printf "$reset"; fi)" ; }
standout() { command printf "$standout$*$(if [ -n "$1" ]; then command printf "$reset"; fi)" ; }
# Ignore "parameters are never passed"
# shellcheck disable=SC2120
reset() { command printf "$reset$*$(if [ -n "$1" ]; then command printf "$reset"; fi)" ; }
bg_black() { command printf "$bg_black$*$(if [ -n "$1" ]; then command printf "$reset"; fi)" ; }
bg_blue() { command printf "$bg_blue$*$(if [ -n "$1" ]; then command printf "$reset"; fi)" ; }
bg_cyan() { command printf "$bg_cyan$*$(if [ -n "$1" ]; then command printf "$reset"; fi)" ; }
bg_green() { command printf "$bg_green$*$(if [ -n "$1" ]; then command printf "$reset"; fi)" ; }
bg_magenta() { command printf "$bg_magenta$*$(if [ -n "$1" ]; then command printf "$reset"; fi)" ; }
bg_red() { command printf "$bg_red$*$(if [ -n "$1" ]; then command printf "$reset"; fi)" ; }
bg_white() { command printf "$bg_white$*$(if [ -n "$1" ]; then command printf "$reset"; fi)" ; }
bg_yellow() { command printf "$bg_yellow$*$(if [ -n "$1" ]; then command printf "$reset"; fi)" ; }
fg_black() { command printf "$fg_black$*$(if [ -n "$1" ]; then command printf "$reset"; fi)" ; }
fg_blue() { command printf "$fg_blue$*$(if [ -n "$1" ]; then command printf "$reset"; fi)" ; }
fg_cyan() { command printf "$fg_cyan$*$(if [ -n "$1" ]; then command printf "$reset"; fi)" ; }
fg_green() { command printf "$fg_green$*$(if [ -n "$1" ]; then command printf "$reset"; fi)" ; }
fg_magenta() { command printf "$fg_magenta$*$(if [ -n "$1" ]; then command printf "$reset"; fi)" ; }
fg_red() { command printf "$fg_red$*$(if [ -n "$1" ]; then command printf "$reset"; fi)" ; }
fg_white() { command printf "$fg_white$*$(if [ -n "$1" ]; then command printf "$reset"; fi)" ; }
fg_yellow() { command printf "$fg_yellow$*$(if [ -n "$1" ]; then command printf "$reset"; fi)" ; }

# Intentionally using variables in format string
# shellcheck disable=SC2059
info() { printf "$*\\n" ; }
# Intentionally using variables in format string
# shellcheck disable=SC2059
warn() {
  increase_indent
  printf "$fg_yellow$*$reset\\n"
  decrease_indent
}
# Intentionally using variables in format string
# shellcheck disable=SC2059
error() {
  increase_indent
  printf "$fg_red$*$reset\\n"
  decrease_indent
}
# Intentionally using variables in format string
# shellcheck disable=SC2059
success() { printf "$fg_green$*$reset\\n" ; }
# Ignore 'arguments are never passed'
# shellcheck disable=SC2120
prompt() {
  if [ "$1" = 'n' ]; then
    command printf "y/$(fg_red '[n]'): "
  else
    command printf "$(fg_green '[y]')/n: "
  fi
}

separator() { printf "===================================================\\n" ; }

banner()
{
  printf "\\n"
  separator
  printf "| %s\\n" "$*" ;
  separator
}

usage()
{
  increase_indent
  USAGE=$(cat <<EOF
Usage:
  $(fg_yellow '-v, --version')
      Defines the version of the agent.
      If not provided, this will default to the latest version.

  $(fg_yellow '-i, --install-dir')
      Defines the install directory of the agent.
      If not provided, this will default to an OS specific location.

  $(fg_yellow '-u, --service-user')
      Defines the service user that will run the agent as a service.
      If not provided, this will default to root.

EOF
  )
  info "$USAGE"
  decrease_indent
  return 0
}

force_exit()
{
  # Exit regardless of subshell level with no "Terminated" message
  kill -PIPE $$
  # Call exit to handle special circumstances (like running script during docker container build)
  exit 1
}

error_exit()
{
  line_num=$(if [ -n "$1" ]; then command printf ":$1"; fi)
  error "ERROR ($SCRIPT_NAME$line_num): ${2:-Unknown Error}" >&2
  shift 2
  if [ -n "$0" ]; then
    increase_indent
    error "$*"
    decrease_indent
  fi
  force_exit
}

print_prereq_line()
{
  if [ -n "$2" ]; then
    command printf "\\n${indent}  - "
    command printf "[$1]: $2"
  fi
}

check_failure()
{
  if [ "$indent" != '' ]; then increase_indent; fi
  command printf "${indent}${fg_red}ERROR: %s check failed!${reset}" "$1"

  print_prereq_line "Issue" "$2"
  print_prereq_line "Resolution" "$3"
  print_prereq_line "Help Link" "$4"
  print_prereq_line "Rerun" "$5"

  command printf "\\n"
  if [ "$indent" != '' ]; then decrease_indent; fi
  force_exit
}

succeeded()
{
  increase_indent
  success "Succeeded!"
  decrease_indent
}

failed()
{
  error "Failed!"
}

# This will set all installation variables
# at the beginning of the script.
setup_installation()
{
    banner "Configuring Installation Variables"
    increase_indent

    # Installation variables
    set_os
    set_download_urls
    set_install_dir
    set_agent_home

    # Service variables
    set_service_user
    set_agent_binary
    set_agent_log
    set_agent_database

    success "Configuration complete!"
    decrease_indent
}

# This will set the os based on the current runtime environment.
# Accepted values are darwin and linux. This value cannot be overriden.
set_os()
{
  os_key=$(uname -s)
  case "$os_key" in
    Darwin)
      os="darwin"
      ;;
    Linux)
      os="linux"
      ;;
    *)
      error "Unsupported os type: $os_key"
      ;;
  esac
}

# This will set the urls to use when downloading the agent and its plugins.
# These urls are constructed based on the --version flag or STANZA_VERSION env variable.
# If not specified, the version defaults to "latest".
set_download_urls()
{
  if [ -z "$version" ] ; then
    # shellcheck disable=SC2153
    version=$STANZA_VERSION
  fi

  if [ -z "$version" ] ; then
    agent_download_url="$DOWNLOAD_BASE/latest/download/${BINARY_NAME}_${os}_amd64"
    plugins_download_url="$DOWNLOAD_BASE/latest/download/${PLUGINS_PACKAGE}"
  else
    agent_download_url="$DOWNLOAD_BASE/download/$version/${BINARY_NAME}_${os}_amd64"
    plugins_download_url="$DOWNLOAD_BASE/download/$version/${PLUGINS_PACKAGE}"
  fi
}

# This will set the install directory of the agent.
# It is set by the --install-dir flag or STANZA_INSTALL_DIR env variable.
# If not specified, it defaults to an OS specific value.
set_install_dir()
{
  if [ -z "$install_dir" ]; then
    # shellcheck disable=SC2153
    install_dir=$STANZA_INSTALL_DIR
  fi

  if [ -z "$install_dir" ]; then
    case "$os" in
      darwin)
        install_dir=${HOME}
        ;;
      linux)
        install_dir=/opt
        ;;
    esac
  fi
}

# This will set agent_home, which is required to run the agent.
# The install directory must be set prior to this.
set_agent_home()
{
  agent_home="$install_dir/observiq/stanza"
}

# This will set the user assigned to the agent service.
# It is set by the --service-user flag or STANZA_SERVICE_USER env variable.
# If not specified, it defaults to root.
set_service_user()
{
  if [ -z "$service_user" ]; then
    # shellcheck disable=SC2153
    service_user=$STANZA_SERVICE_USER
  fi

  if [ -z "$service_user" ] ; then
    service_user="root"
  fi
}

# This will set the location of the binary used to launch the agent.
# This value cannot be overriden and is based on the location of agent_home.
set_agent_binary()
{
  agent_binary="$agent_home/$BINARY_NAME"
}

# This will set the agent log location.
set_agent_log()
{
  agent_log="$agent_home/$SERVICE_NAME.log"
}

# This will set the agent database file.
set_agent_database()
{
  agent_database="$agent_home/$SERVICE_NAME.db"
}

# This will check all prerequisites before running an installation.
check_prereqs()
{
  banner "Checking Prerequisites"
  increase_indent
  os_check
  os_arch_check
  dependencies_check
  success "Prerequisite check complete!"
  decrease_indent
}

# This will check if the operating system is supported.
os_check()
{
  info "Checking that the operating system is supported..."
  os_type=$(uname -s)
  case "$os_type" in
    Darwin|Linux)
      succeeded
      ;;
    *)
      failed
      error_exit "The operating system $(fg_yellow "$os_type") is not supported by this script."
      ;;
  esac
}

# This will check if the system architecture is supported.
os_arch_check()
{
  info "Checking for valid operating system architecture..."
  os_arch=$(uname -m)
  if [ "$os_arch" = 'x86_64' ]; then
    succeeded
  else
    failed
    error_exit "The operating system architecture $(fg_yellow "$os_arch") is not supported by this script."
  fi
}

# This will check if the current environment has
# all required shell dependencies to run the installation.
dependencies_check()
{
  info "Checking for script dependencies..."
  FAILED_PREREQS=''
  for prerequisite in $PREREQS; do
    if command -v "$prerequisite" >/dev/null; then
      continue
    else
      if [ -z "$FAILED_PREREQS" ]; then
        FAILED_PREREQS="${fg_red}$prerequisite${reset}"
      else
        FAILED_PREREQS="$FAILED_PREREQS, ${fg_red}$prerequisite${reset}"
      fi
    fi
  done

  if [ -n "$FAILED_PREREQS" ]; then
    failed
    error_exit "The following dependencies are required by this script: [$FAILED_PREREQS]"
  fi
  succeeded
  return 0
}

# This will install the package by downloading the archived agent,
# extracting the binaries, and then removing the archive.
install_package()
{
  banner "Installing Stanza"
  increase_indent

  info "Creating Stanza directory..."
  mkdir -p "$agent_home"
  succeeded

  info "Checking that service is not running..."
  stop_service
  succeeded

  info "Downloading binary..."
  curl -L "$agent_download_url" -o "$agent_binary" --progress-bar --fail || error_exit "$LINENO" "Failed to download package"
  succeeded

  info "Setting permissions..."
  chmod +x "$agent_binary"
  ln -sf "$agent_binary" "/usr/local/bin/$BINARY_NAME"
  succeeded

  info "Downloading plugins..."
  mkdir -p "$agent_home/tmp"
  curl -L "$plugins_download_url" -o "$agent_home/tmp/plugins.tar.gz" --progress-bar --fail || error_exit "$LINENO" "Failed to download plugins"
  succeeded

  info "Extracting plugins..."
  tar -zxf "$agent_home/tmp/plugins.tar.gz" -C "$agent_home"
  rm -fr "$agent_home/tmp"

  success "Stanza installation complete!"
  decrease_indent
}

# This will create the agent config as a YAML file.
generate_config()
{
  banner "Generating Config"
  increase_indent

  info "Creating config file..."
  config_file="$agent_home/config.yaml"
  create_config_file "$config_file"
  succeeded

  success "Generation complete!"
  decrease_indent
}

# This will the create a config file with an example pipeline.
create_config_file()
{
  # Don't overwrite a config file that already exists
  if [ -f "$1" ] ; then
    return
  fi

  cat << EOF > "$1"
pipeline:
  # An example input that generates a single log entry when Stanza starts up.
  - type: generate_input
    count: 1
    entry:
      record: This is a sample log generated by Stanza
    output: example_output

  # An example input that monitors the contents of a file.
  # For more info: https://github.com/opentelemetry/opentelemetry-log-collection/blob/master/docs/operators/file_input.md
  #
  # - type: file_input
  #   include:
  #     - /sample/file/path
  #   output: example_output

  # An example output that sends captured logs to stdout.
  - id: example_output
    type: stdout

  # An example output that sends captured logs to google cloud logging.
  # For more info: https://github.com/opentelemetry/opentelemetry-log-collection/blob/master/docs/operators/google_cloud_output.md
  #
  # - id: example_output
  #   type: google_cloud_output
  #   credentials_file: /my/credentials/file

  # An example output that sends captured logs to elasticsearch.
  # For more info: https://github.com/opentelemetry/opentelemetry-log-collection/blob/master/docs/operators/elastic_output.md
  #
  # - id: example_output
  #   type: elastic_output
  #   addresses:
  #     - http://my_node_address:9200
  #   api_key: my_api_key
EOF
}

# This will install the service by detecting the init system
# and configuring the launcher to run accordinngly
install_service()
{
  banner "Creating Service"
  increase_indent

  service_type="$(init_type)"
  case "$service_type" in
    launchd)
      create_launchd_service
      ;;
    sysv|upstart)
      create_sysv_service
      ;;
    systemd)
      create_systemd_service
      ;;
    *)
      error "Your init system ($fg_yellow$service_type$fg_red) is not supported."
      error "The agent must be started manually by running $agent_binary"
      service_install_failed="true"
  esac

  if [ "$service_install_failed" = "true" ] ; then
    error "Failed to install service"
  else
    success "Service installation complete"
  fi
  decrease_indent
}

# This is used to discover the init system for a unix environment. It supports
# launchd, upstart, systemd, and sysv.
init_type()
{
  if [ "$os" = darwin ]; then
    command printf "launchd"
    return
  fi

  ubuntu_test="$(ubuntu_init_type)"
  if [ "$ubuntu_test" != "unknown" ]; then
    command printf "$ubuntu_test"
    return
  fi

  upstart_test="$( (/sbin/init --version || :) 2>&1)"
  if command printf "$upstart_test" | grep -q 'upstart'; then
    command printf "upstart"
    return
  fi

  systemd_test="$(systemctl || : 2>&1)"
  if command printf "$systemd_test" | grep -q '\-.mount'; then
    command printf "systemd"
    return
  fi

  if [ -f /etc/init.d/cron ] && [ ! -L /etc/init.d/cron ]; then
    command printf "sysv"
    return
  fi

  command printf "unknown"
  return
}

# This exists because Ubuntu (at least 16.04 LTS) has both upstart and systemd installed. If this machine
# is running Ubuntu, check which of those systems is being used. If it's not running Ubuntu, then just
# return "unknown", which will tell the calling function to continue with the other tests
ubuntu_init_type()
{
  if uname -a | grep -q Ubuntu; then
    # shellcheck disable=SC2009
    if ps -p1 | grep -q systemd; then
      command printf 'systemd'
    else
      command printf 'upstart'
    fi
  else
    command printf "unknown"
  fi
}

# This will detect the service type and stop it
stop_service()
{
  service_type="$(init_type)"
  case "$service_type" in
    launchd)
      stop_launchd_service
      ;;
    sysv|upstart)
      stop_sysv_service
      ;;
    systemd)
      stop_systemd_service
      ;;
  esac
}

# This will configure the agent to run as a service with launchd.
create_launchd_service()
{
  PLISTFILE="${HOME}/Library/LaunchAgents/com.observiq.${SERVICE_NAME}.plist"
  replace_service="false"

  if [ -e "$PLISTFILE" ]; then
    request_service_replacement
    if [ $replace_service = "true" ]; then
      launchctl stop "com.observiq.${SERVICE_NAME}" || warn "Failed to stop service"
      launchctl unload "${PLISTFILE}" 2>/dev/null
    else
      return 0
    fi
  fi

  mkdir -p "${HOME}/Library/LaunchAgents"
  info "Creating service file..."
  create_launchd_file "$PLISTFILE"
  succeeded

  info "Loading service file..."
  launchctl load "$PLISTFILE" 2>/dev/null
  succeeded

  info "Starting service..."
  start_launchd_service
  succeeded

  startup_cmd="launchctl start com.observiq.$SERVICE_NAME"
  shutdown_cmd="launchctl stop com.observiq.$SERVICE_NAME"
}

# This will create the launchd plist file.
create_launchd_file()
{
  cat > "$1" << PLISTFILECON
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
  <dict>
    <key>Label</key>
    <string>com.observiq.${SERVICE_NAME}</string>
    <key>Program</key>
    <string>$agent_binary</string>
    <key>ProgramArguments</key>
    <array>
      <string>$agent_binary</string>
      <string>--log_file</string>
      <string>$agent_log</string>
      <string>--database</string>
      <string>$agent_database</string>
    </array>
    <key>WorkingDirectory</key>
    <string>$agent_home</string>
    <key>RunAtLoad</key>
    <true/>
    <key>SessionCreate</key>
    <true/>
    <key>UserName</key>
    <string>$service_user</string>
  </dict>
</plist>
PLISTFILECON
}

# This will start the launchd service. It will fail
# if unsuccessful.
start_launchd_service()
{
  launchctl start "com.observiq.${SERVICE_NAME}"

  RET="$?"
  if [ "$RET" -eq 3 ]; then
    error_exit $LINENO "launchctl is unable to start the $SERVICE_NAME service unless the user is logged in via a GUI."
  elif [ "$RET" -ne 0 ]; then
    error_exit $LINENO "An error occurred while attempting to start the service"
  fi
}

# This will stop the launchd service
stop_launchd_service()
{
  launchctl stop "com.observiq.${SERVICE_NAME}" >/dev/null 2>&1 || true
}

# This will configure the launcher to run as a service with sysv
create_sysv_service()
{
  sysv_file="/etc/init.d/$SERVICE_NAME"
  replace_service="false"

  if [ -e "/etc/init.d/$SERVICE_NAME" ]; then
    request_service_replacement
    if [ $replace_service = "false" ]; then
      return 0
    fi
  fi

  info "Creating sevice file..."
  create_sysv_file $sysv_file
  chmod 755 $sysv_file
  succeeded


  info "Adding service..."
  add_sysv_service
  succeeded

  if [ $replace_service = "true" ]; then
    info "Restarting service..."
    restart_sysv_service
    succeeded
  else
    info "Starting service..."
    start_sysv_service
    succeeded
  fi

  startup_cmd="service $SERVICE_NAME start"
  shutdown_cmd="service $SERVICE_NAME stop"
  return 0
}

# This will create the sysv file used to run
# the agent as a service.
create_sysv_file()
{
  cat << "EOF" > "$1"
#!/bin/sh
# stanza daemon
# chkconfig: 2345 99 05
# description: stanza log agent
# processname: REPLACE_AGENT_BINARY
# pidfile: /var/run/log-agent.pid

# Source function library.
if [ -e /etc/init.d/functions ]; then
  STATUS=true
  . /etc/init.d/functions
fi

if [ -e /lib/lsb/init-functions ]; then
  PROC=true
  . /lib/lsb/init-functions
fi

# Pull in sysconfig settings
[ -f /etc/sysconfig/log-agent ] && . /etc/sysconfig/log-agent

PROGRAM=log-agent
LOCKFILE=/var/lock/$PROGRAM
PIDFILE=/var/run/log-agent.pid
DEBUG=false
RETVAL=0

start() {
    if [ -f $PIDFILE ]; then
        PID=$(cat $PIDFILE)
        echo " * $PROGRAM already running: $PID"
        RETVAL=2
    else
        echo " * Starting $PROGRAM"
        if [ -n "REPLACE_SERVICE_USER" ]; then
          su -p REPLACE_SERVICE_USER -c "nohup REPLACE_AGENT_BINARY --log_file REPLACE_AGENT_LOG --database REPLACE_AGENT_DATABASE" > /dev/null 2>&1 &
        else
          nohup "REPLACE_AGENT_BINARY --log_file REPLACE_AGENT_LOG --database REPLACE_AGENT_DATABASE" > /dev/null 2>&1 &
        fi
        echo $! > $PIDFILE
        RETVAL=$?
        [ "$RETVAL" -eq 0 ] && touch $LOCKFILE
    fi
}

stop() {
    if [ -f $PIDFILE ]; then
        PID=$(cat $PIDFILE);
        printf " * Stopping $PROGRAM... "
        kill $PID > /dev/null 2>&1
        echo "stopped"
        rm $PIDFILE && rm -f $LOCKFILE
        RETVAL=0
    else
        echo " * $PROGRAM is not running"
        RETVAL=3
    fi
}

pid_status() {
  if [ -e "$PIDFILE" ]; then
      echo " * $PROGRAM" is running, pid=`cat "$PIDFILE"`
      RETVAL=0
  else
      echo " * $PROGRAM is not running"
      RETVAL=1
  fi
}

agent_status() {
   if [ $PROC ]; then
     status_of_proc -p $PIDFILE "$PROGRAM" "$PROGRAM"
     RETVAL=$?
   elif [ $STATUS ]; then
     status -p $PIDFILE $PROGRAM
     RETVAL=$?
   else
     pid_status
   fi
}

case "$1" in
    start)
        start
        ;;
    stop)
        stop
        ;;
    status)
        agent_status
        ;;
    restart)
        stop
        start
        ;;
    *)
        echo "Usage: {start|stop|status|restart}"
        RETVAL=3
        ;;
esac
exit $RETVAL
EOF
  sed -i "s|REPLACE_AGENT_HOME|$agent_home|" "$1"
  sed -i "s|REPLACE_SERVICE_USER|$service_user|" "$1"
  sed -i "s|REPLACE_AGENT_BINARY|$agent_binary|" "$1"
  sed -i "s|REPLACE_AGENT_LOG|$agent_log|" "$1"
  sed -i "s|REPLACE_AGENT_DATABASE|$agent_database|" "$1"
}

# This will load the sysv service.
add_sysv_service()
{
  if command -v "chkconfig" > /dev/null ; then
    chkconfig --add "$SERVICE_NAME" || error_exit "$LINENO" "Failed to install service"
  elif command -v "update-rc.d" > /dev/null ; then
    update-rc.d "$SERVICE_NAME" defaults
  else
    error "Could not find$fg_yellow chkconfig$fg_red or$fg_yellow update-rd.c$fg_red"
    error "The agent has been extracted to $fg_blue$agent_home$fg_red and configured."
  fi
}

# This will start the sysv service. It will fail
# and if unsuccessful.
start_sysv_service()
{
  if ! output="$(service "$SERVICE_NAME" start 2>&1)"; then
    error_exit "$LINENO" "Failed to start service:" "$output"
  fi
}

# This will stop the sysv service if it is running
stop_sysv_service()
{
  service "$SERVICE_NAME" stop >/dev/null 2>&1  || true
}

# This will restart the sysv service. It will fail
# and if unsuccessful.
restart_sysv_service()
{
  if ! output="$(service "$SERVICE_NAME" restart 2>&1)"; then
    error_exit "$LINENO" "Failed to start service:" "$output"
  fi
}

# This will configure the launcher to run as a service with systemd
create_systemd_service()
{
  systemd_file="/etc/systemd/system/$SERVICE_NAME.service"
  replace_service="false"

  if [ -e $systemd_file ]; then
    request_service_replacement
    if [ $replace_service = "false" ]; then
      return 0
    fi
  fi

  info "Creating service file..."
  create_systemd_file $systemd_file
  chmod 644 $systemd_file
  succeeded

  info "Reloading systemd configuration..."
  systemctl daemon-reload || error_exit "$LINENO" "Failed to reload services"
  succeeded

  info "Enabling service..."
  systemctl enable "$SERVICE_NAME.service" >/dev/null 2>&1 || error_exit "$LINENO" "Failed to enable service"
  succeeded

  if [ $replace_service = "true" ]; then
    info "Restarting service..."
    restart_systemd_service
    succeeded
  else
    info "Starting service..."
    start_systemd_service
    succeeded
  fi

  shutdown_cmd="systemctl stop $SERVICE_NAME"
  startup_cmd="systemctl start $SERVICE_NAME"
  return 0
}

# This will create a systemd service file. The first argument
# represents the designated file location.
create_systemd_file()
{
  cat << EOF > "$1"
[Unit]
Description=Stanza Log Agent
After=network.target

[Service]
Type=simple
PIDFile=/tmp/log-agent.pid
User=$service_user
Group=$service_user
Environment=PATH=/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin
WorkingDirectory=$agent_home
ExecStart=$agent_binary --log_file $agent_log --database $agent_database
SuccessExitStatus=143
TimeoutSec=0
StandardOutput=null

[Install]
WantedBy=multi-user.target
EOF
}

# This will start the systemd service. It will fail
# and disable the service if unsuccessful.
start_systemd_service()
{
  if ! systemctl start "$SERVICE_NAME.service"; then
    error "Failed to start $SERVICE_NAME.service. Disabling $SERVICE_NAME.service."
    systemctl disable "$SERVICE_NAME.service"
    error_exit "$LINENO" "Failed to start service"
  fi
}

# This will stop the systemd service if it is running
stop_systemd_service()
{
  systemctl stop "$SERVICE_NAME.service" >/dev/null 2>&1 || true
}

# This will restart the systemd service. It will fail
# if unsuccessful.
restart_systemd_service()
{
  systemctl restart "$SERVICE_NAME.service" || error_exit "$LINENO" "Failed to restart service"
}

# This will notify the user that a service already exists
# and will await their response on replacing the service
request_service_replacement()
{
  command printf "${indent}Service '$(fg_cyan $SERVICE_NAME)' already exists. Replace it? $(prompt)"
  read -r replace_service_response
  case $replace_service_response in
    n|N|no|No|NO)
      warn "Skipping service creation!"
      replace_service="false"
      ;;
    *)
      increase_indent
      success "Replacing service!"
      decrease_indent
      replace_service="true"
      ;;
  esac
}

# This will display the results of an installation
display_results()
{
    banner 'Information'
    increase_indent
    info "Stanza Home:     $(fg_cyan "$agent_home")$(reset)"
    info "Stanza Config:   $(fg_cyan "$agent_home/config.yaml")$(reset)"
    info "Start Command:  $(fg_cyan "$startup_cmd")$(reset)"
    info "Stop Command:   $(fg_cyan "$shutdown_cmd")$(reset)"
    decrease_indent

    banner 'Troubleshooting'
    increase_indent
    info "To troubleshoot issues, stanza can be run manually for faster iteration."
    info "1) Stop the stanza service: $(fg_cyan "$shutdown_cmd")"
    info "2) Navigate to the stanza home directory: $(fg_cyan "cd $agent_home")"
    info "3) Run stanza manually: $(fg_cyan "./stanza --debug")"
    decrease_indent

    banner "$(fg_green Installation Complete!)"
    return 0
}

main()
{
  if [ $# -ge 1 ]; then
    while [ -n "$1" ]; do
      case "$1" in
        -y|--accept-defaults)
          accept_defaults="yes" ; shift 1 ;;
        -v|--version)
          version=$2 ; shift 2 ;;
        -i|--install-dir)
          install_dir=$2 ; shift 2 ;;
        -u|--service-user)
          service_user=$2 ; shift 2 ;;
        -h|--help)
          usage
          force_exit
          ;;
      --)
        shift; break ;;
      *)
        error "Invalid argument: $1"
        usage
        force_exit
        ;;
      esac
    done
  fi

  check_prereqs
  setup_installation
  install_package
  generate_config
  install_service
  display_results
}

main "$@"
