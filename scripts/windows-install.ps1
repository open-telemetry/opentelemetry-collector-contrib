new-module -name LogAgentInstall -scriptblock {

  # Constants
  $DEFAULT_WINDOW_TITLE = $host.ui.rawui.WindowTitle
  $DEFAULT_INSTALL_PATH = 'C:\'
  $DOWNLOAD_BASE = "https://github.com/opentelemetry/opentelemetry-log-collection/releases"
  $SERVICE_NAME = 'stanza'
  $INDENT_WIDTH = '  '
  $MIN_DOT_NET_VERSION = '4.5'

  # Functions
  function Set-Variables {
    if ($host.name -NotMatch "ISE") {
      $IS_PS_ISE = $false
      $script:DEFAULT_FG_COLOR = $host.ui.rawui.ForegroundColor
      $script:DEFAULT_BG_COLOR = $host.ui.rawui.BackgroundColor
    }
    else {
      $script:IS_PS_ISE = $true
      $script:DEFAULT_FG_COLOR = [System.ConsoleColor]::White
      $script:DEFAULT_BG_COLOR = $psIse.Options.ConsolePaneBackgroundColor
    }
  }

  function Add-Indent {
    $script:indent = "${script:indent}$INDENT_WIDTH"
  }

  function Remove-Indent {
    $script:indent = $script:indent -replace "^$INDENT_WIDTH", ''
  }

  # Takes a variable amount of alternating strings and their colors.
  # An empty-string color uses the default text color.
  # The last given string does not require a color (uses default)
  # e.g.: string1 color1 string2 color2 string3
  function Show-ColorText {
    Write-Host "$indent" -NoNewline
    for ($i = 0; $i -lt $args.count; $i++) {
      $message = $args[$i]
      $i++
      $color = $args[$i]
      if (!$color) {
        $color = $script:DEFAULT_FG_COLOR
      }
      Write-Host "$message" -ForegroundColor "$color" -NoNewline
    }
    Write-Host ""
  }

  function Show-Separator {
    Show-ColorText "============================================" $args
  }

  function Show-Header {
    $message, $color = $args
    Show-ColorText ""
    Show-Separator
    Show-ColorText '| ' '' "$message" $color
    Show-Separator
  }

  function Set-Window-Title {
    $host.ui.rawui.windowtitle = "Stanza Install"
  }

  function Restore-Window-Title {
    $host.ui.rawui.windowtitle = $DEFAULT_WINDOW_TITLE
  }

  function Complete {
    Show-ColorText "Complete" DarkGray
  }

  function Succeeded {
    Add-Indent
    Show-ColorText "Succeeded!" Green
    Remove-Indent
  }

  function Failed {
    Add-Indent
    Show-ColorText "Failed!" Red
    Remove-Indent
  }

  function Show-Usage {
    Add-Indent
    Show-ColorText 'Options:'
    Add-Indent

    Show-ColorText ''
    Show-ColorText '-y, --accept_defaults'
    Add-Indent
    Show-ColorText 'Accepts all default values for installation.' DarkCyan
    Remove-Indent

    Show-ColorText ''
    Show-ColorText '-v, --version'
    Add-Indent
    Show-ColorText 'Defines the version of the agent.' DarkCyan
    Show-ColorText 'If not provided, this will default to the latest version.' DarkCyan
    Remove-Indent

    Show-ColorText ''
    Show-ColorText '-i, --install_dir'
    Add-Indent
    Show-ColorText 'Defines the install directory of the agent.' DarkCyan
    Show-ColorText 'If not provided, this will default to C:/observiq.' DarkCyan
    Remove-Indent

    Show-ColorText ''
    Show-ColorText '-u, --service_user'
    Add-Indent
    Show-ColorText 'Defines the service user that will run the agent.' DarkCyan
    Show-ColorText 'If not provided, this will default to the current user.' DarkCyan
    Remove-Indent

    Remove-Indent
    Remove-Indent
  }

  function Exit-Error {
    if ($indent) { Add-Indent }
    $line_number = $args[0]
    $message = ""
    if ($args[1]) { $message += "`n${indent}- [Issue]: $($args[1])" }
    if ($args[2]) { $message += "`n${indent}- [Resolution]: $($args[2])" }
    if ($args[3]) { $message += "`n${indent}- [Help Link]: $($args[3])" }
    if ($args[4]) {
      $message += "`n${indent}- [Rerun]: $($args[4])"
    }
    elseif ($script:rerun_command) {
      $message += "`n${indent}- [Rerun]: $script:rerun_command"
    }
    throw "Error (windows_install.ps1:${line_number}): $message"
    if ($indent) { Remove-Indent }
  }

  function Get-AbsolutePath ($path) {
    $path = [System.IO.Path]::Combine(((Get-Location).Path), ($path))
    $path = [System.IO.Path]::GetFullPath($path)
    return $path;
  }

  function Request-Confirmation ($default = "y") {
    if ($default -eq "n") {
      Write-Host -NoNewline "y/"
      Write-Host -NoNewline -ForegroundColor Red "[n]: "
    }
    else {
      Write-Host -NoNewline -ForegroundColor Green "[y]"
      Write-Host -NoNewline "/n: "
    }
  }

  # This will check for all required conditions
  # before executing an installation.
  function Test-Prerequisites {
    Show-Header "Checking Prerequisites"
    Add-Indent
    Test-PowerShell
    Test-Architecture
    Test-DotNet
    Complete
    Remove-Indent
  }

  # This will ensure that the script is executed
  # in the correct version of PowerShell.
  function Test-PowerShell {
    Show-ColorText  "Checking PowerShell... "
    if (!$ENV:OS) {
      Failed
      Exit-Error $MyInvocation.ScriptLineNumber 'Install script was not executed in PowerShell.'
    }

    $script:PSVersion = $PSVersionTable.PSVersion.Major
    if ($script:PSVersion -lt 3) {
      Failed
      Exit-Error $MyInvocation.ScriptLineNumber 'Your PowerShell version is not supported.' 'Please update to PowerShell 3+.'
    }
    Succeeded
  }


  # This will ensure that the CPU Architecture is supported.
  function Test-Architecture {
    Show-ColorText 'Checking CPU Architecture... '
    if ([System.IntPtr]::Size -eq 4) {
      Failed
      Exit-Error $MyInvocation.ScriptLineNumber '32-bit Operating Systems are currently not supported.'
    }
    else {
      Succeeded
    }
  }

  # This will ensure that the version of .NET is supported.
  function Test-DotNet {
    Show-ColorText 'Checking .NET Framework Version...'
    if ([System.Version](Get-ChildItem 'HKLM:\SOFTWARE\Microsoft\NET Framework Setup\NDP' -recurse | Get-ItemProperty -name Version, Release -EA 0 | Where-Object { $_.PSChildName -match '^(?!S)\p{L}' } | Sort-Object -Property Version -Descending | Select-Object Version -First 1).version -ge [System.Version]"$MIN_DOT_NET_VERSION") {
      Succeeded
    }
    else {
      Failed
      Exit-Error $MyInvocation.ScriptLineNumber ".NET Framework $MIN_DOT_NET_VERSION is required." "Install .NET Framework $MIN_DOT_NET_VERSION or later"
    }
  }

  # This will set the values of all installation variables.
  function Set-InstallVariables {
    Show-Header "Configuring Installation Variables"
    Add-Indent
    Set-Defaults
    Set-DownloadURLs
    Set-InstallDir
    Set-HomeDir
    Set-PluginDir
    Set-BinaryLocation
    Set-ServiceUser

    Complete
    Remove-Indent
  }

  # This will prompt a user to use default values. If yes, this
  # will set the install_dir, agent_name, and service_user to their
  # default values.
  function Set-Defaults {
    If ( !$script:accept_defaults ) {
      Write-Host -NoNewline "${indent}Accept installation default values? "
      Request-Confirmation
      $script:accept_defaults = Read-Host
    }
    Else {
      $accepted_defaults_via_args = "true"
    }

    Switch ( $script:accept_defaults.ToLower()) {
      { ($_ -in "n", "no") } {
        return
      }
      default {
        If (!$accepted_defaults_via_args) { Write-Host -NoNewline "${indent}" }
        Show-ColorText "Using default installation values" Green
        If (!$script:install_dir) {
          $script:install_dir = $DEFAULT_INSTALL_PATH
        }
        If (!$script:service_user) {
          $script:service_user = [System.Security.Principal.WindowsIdentity]::GetCurrent().Name
        }
      }
    }
  }

  # This will set the install path of the agent. If not provided as a flag,
  # the user will be prompted for this information.
  function Set-InstallDir {
    Show-ColorText 'Setting install directory...'
    Add-Indent
    If ( !$script:install_dir ) {
      Write-Host -NoNewline "${indent}Install path   ["
      Write-Host -NoNewline -ForegroundColor Cyan "$DEFAULT_INSTALL_PATH"
      Write-Host -NoNewline ']: '
      $script:install_dir = Read-Host
      If ( !$script:install_dir ) {
        $script:install_dir = $DEFAULT_INSTALL_PATH
      }
      $script:install_dir = Get-AbsolutePath($script:install_dir)
    }
    else {
      $script:install_dir = [System.IO.Path]::GetFullPath($script:install_dir)
    }

    If (-Not (Test-Path $script:install_dir) ) {
      New-Item -ItemType directory -Path $script:install_dir | Out-Null
    }

    Show-ColorText 'Using install directory: ' '' "$script:install_dir" DarkCyan
    Remove-Indent
  }

  # This will set the urls to use when downloading the agent and its plugins.
  # These urls are constructed based on the --version flag.
  # If not specified, the version defaults to "latest".
  function Set-DownloadURLs {
    Show-ColorText 'Configuring download urls...'
    Add-Indent
    if ( !$script:version ) {
      $script:agent_download_url = "$DOWNLOAD_BASE/latest/download/stanza_windows_amd64"
      $script:plugins_download_url = "$DOWNLOAD_BASE/latest/download/stanza-plugins.zip"
    }
    else {
      $script:agent_download_url = "$DOWNLOAD_BASE/download/$script:version/stanza_windows_amd64"
      $script:plugins_download_url = "$DOWNLOAD_BASE/download/$script:version/stanza-plugins.zip"
    }
    Show-ColorText "Using agent download url: " '' "$script:agent_download_url" DarkCyan
    Show-ColorText "Using plugins download url: " '' "$script:plugins_download_url" DarkCyan
    Remove-Indent
  }

  # This will set the home directory of the agent based on
  # the install directory provided.
  function Set-HomeDir {
    Show-ColorText 'Setting home directory...'
    Add-Indent
    $script:agent_home = "{0}observiq\stanza" -f $script:install_dir

    If (-Not (Test-Path $script:agent_home) ) {
      New-Item -ItemType directory -Path $agent_home | Out-Null
    }

    Show-ColorText "Using home directory: " '' "$script:agent_home" DarkCyan
    Remove-Indent
  }

  # This will set the plugins directory of the agent based on
  # the install directory provided.
  function Set-PluginDir {
    Show-ColorText 'Setting plugin directory...'
    Add-Indent
    $script:plugin_dir = "{0}\plugins" -f $script:agent_home

    If (-Not (Test-Path $script:plugin_dir) ) {
      New-Item -ItemType directory -Path $plugin_dir | Out-Null
    }

    Show-ColorText "Using plugin directory: " '' "$script:plugin_dir" DarkCyan
    Remove-Indent
  }

  # This will set the path for the agent binary.
  function Set-BinaryLocation {
    Show-ColorText 'Setting binary location...'
    Add-Indent

    $script:binary_location = Get-AbsolutePath("$script:agent_home\stanza.exe")
    Show-ColorText "Using binary location: " '' "$script:binary_location" DarkCyan
    Remove-Indent
  }

  # This will set the user that will run the agent as a service.
  # If not provided as a flag, this will default to the current user.
  function Set-ServiceUser {
    Show-ColorText 'Setting service user...'
    Add-Indent
    If (!$script:service_user ) {
      $current_user = [System.Security.Principal.WindowsIdentity]::GetCurrent().Name
      $script:service_user = $current_user
    }
    Show-ColorText "Using service user: " '' "$script:service_user" DarkCyan
    Remove-Indent
  }

  # This will set user permissions on the install directory.
  function Set-Permissions {
    Show-Header "Setting Permissions"
    Add-Indent
    Show-ColorText "Setting file permissions for NetworkService user..."

    try {
      $Account = New-Object System.Security.Principal.NTAccount "NT AUTHORITY\NetworkService"
      $InheritanceFlag = [System.Security.AccessControl.InheritanceFlags]::ContainerInherit -bor [System.Security.AccessControl.InheritanceFlags]::ObjectInherit
      $PropagationFlag = 0
      $NewAccessRule = New-Object Security.AccessControl.FileSystemAccessRule $Account, "Modify", $InheritanceFlag, $PropagationFlag, "Allow"
      $FolderAcl = Get-Acl $script:agent_home
      $FolderAcl.SetAccessRule($NewAccessRule)
      $FolderAcl | Set-Acl $script:agent_home
      Succeeded
    }
    catch {
      Failed
      Exit-Error $MyInvocation.ScriptLineNumber "Unable to set file permissions for NetworkService user: $($_.Exception.Message)"
    }

    try {
      $Account = New-Object System.Security.Principal.NTAccount "$script:service_user"

      # First, ensure modify permissions on the install path
      Show-ColorText 'Checking for ' '' 'Modify' Yellow ' permissions...'
      Add-Indent
      $ModifyValue = [System.Security.AccessControl.FileSystemRights]::Modify -as [int]
      $FolderAcl = Get-Acl $script:agent_home

      $UserHasModify = $FolderAcl.Access | Where-Object { ($_.FileSystemRights -band $ModifyValue) -eq $ModifyValue -and $_.IdentityReference -eq $Account }
      if (-not $UserHasModify) {
        Show-ColorText 'Modify' Yellow ' permissions not found for ' '' "$Account" DarkCyan
        Remove-Indent
        Show-ColorText 'Granting permissions...'
        $NewAccessRule = New-Object Security.AccessControl.FileSystemAccessRule $Account, "Modify", $InheritanceFlag, $PropagationFlag, "Allow"
        $FolderAcl.SetAccessRule($NewAccessRule)
        $FolderAcl | Set-Acl $script:agent_home
        Succeeded
      }
      else {
        Show-ColorText "$Account" DarkCyan ' already possesses ' "" 'Modify' Yellow ' permissions on the install directory.'
        Remove-Indent
      }
    }
    catch {
      Show-ColorText "Unable to verify user permissions: $($_.Exception.Message)" Yellow
    }
    Complete
    Remove-Indent
  }

  # This will download the agent binary to the binary location.
  function Get-StanzaBinary {
    Show-Header "Downloading Stanza Binary"
    Add-Indent
    Show-ColorText 'Downloading binary. Please wait...'
    Show-ColorText "$INDENT_WIDTH$script:agent_download_url" DarkCyan ' -> ' '' "$script:binary_location" DarkCyan
    try {
      $WebClient = New-Object System.Net.WebClient
      $WebClient.DownloadFile($script:agent_download_url, $script:binary_location)
      Complete
    }
    catch {
      Failed
      $error_message = $_.Exception.Message -replace 'Exception calling.*?: ', ''
      Exit-Error $MyInvocation.ScriptLineNumber "Failed to download agent binary: $error_message"
    }
    Remove-Indent
  }

  # This will download and extract plugins to the plugins directory.
  function Get-StanzaPlugins {
    Show-Header "Downloading Stanza Plugins"
    Add-Indent
    Show-ColorText 'Downloading plugins. Please wait...'
    Show-ColorText "$INDENT_WIDTH$script:plugins_download_url" DarkCyan ' -> ' '' "$script:agent_home\plugins.zip" DarkCyan

    try {
      New-Item -Path "$script:agent_home\download\tmp" -ItemType "directory"
      Complete
    }
    catch {
      Failed
      $error_message = $_.Exception.Message -replace 'Exception calling.*?: ', ''
      Exit-Error $MyInvocation.ScriptLineNumber "Failed to create tmp directory plugins: $error_message"
    }

    try {
      $WebClient = New-Object System.Net.WebClient
      $WebClient.DownloadFile($script:plugins_download_url, "$script:agent_home\download\plugins.zip")
      Complete
    }
    catch {
      Failed
      $error_message = $_.Exception.Message -replace 'Exception calling.*?: ', ''
      Exit-Error $MyInvocation.ScriptLineNumber "Failed to download plugins: $error_message"
    }

    try {
      [System.Reflection.Assembly]::LoadWithPartialName("System.IO.Compression.FileSystem") | Out-Null
      [System.IO.Compression.ZipFile]::ExtractToDirectory("$script:agent_home\download\plugins.zip", "$script:agent_home\download")
      Complete
    }
    catch {
      Failed
      $error_message = $_.Exception.Message -replace 'Exception calling.*?: ', ''
      Exit-Error $MyInvocation.ScriptLineNumber "Failed to expand plugins archive: $error_message"
    }

    try {
      Copy-Item -Path "$script:agent_home\download\plugins\*.yaml" -Destination "$script:agent_home\plugins"
      Complete
    }
    catch {
      Failed
      $error_message = $_.Exception.Message -replace 'Exception calling.*?: ', ''
      Exit-Error $MyInvocation.ScriptLineNumber "Failed to relocate plugins: $error_message"
    }

    try {
      Remove-Item -Path "$script:agent_home\download" -Recurse
      Complete
    }
    catch {
      Failed
      $error_message = $_.Exception.Message -replace 'Exception calling.*?: ', ''
      Exit-Error $MyInvocation.ScriptLineNumber "Failed to clean up download: $error_message"
    }

    Remove-Indent
  }

  # This will remove the agent service.
  function Remove-AgentService {
    $service = Get-Service $SERVICE_NAME -ErrorAction SilentlyContinue
    If ($service) {
      Show-ColorText 'Previous ' '' "$SERVICE_NAME" DarkCyan ' service detected.'
      If ($service.Status -eq 'Running') {
        Show-ColorText 'Stopping ' '' "$SERVICE_NAME" DarkCyan '...'
        if ($script:PSVersion -ge 5) {
          Stop-Service $SERVICE_NAME -NoWait -Force | Out-Null
        }
        else {
          Stop-Service $SERVICE_NAME | Out-Null
        }
        Show-ColorText "Service Stopped."
      }
      Show-ColorText 'Removing ' '' "$SERVICE_NAME" DarkCyan ' service...'
      (Get-WmiObject win32_service -Filter "name='$SERVICE_NAME'").delete() | Out-Null

      If ( $LASTEXITCODE -eq 0 ) {
        # Sleep 5 seconds to give time for the service to be fully deleted
        sleep -s 5
        Show-ColorText 'Previous service removed.'
      }
    }
  }

  # This will create the agent config.
  function New-AgentConfig {
    Show-Header "Generating Config"
    Add-Indent
    $config_file = "$script:agent_home\config.yaml"
    Show-ColorText 'Writing config file: ' '' "$config_file" DarkCyan
    try {
      Write-Config $config_file
    }
    catch {
      Exit-Error $MyInvocation.ScriptLineNumber "Failed to write config file: $($_.Exception.Message)" 'Please ensure you have permission to create this file.'
    }

    Complete
    Remove-Indent
  }

  # This will write the agent config.
  function Write-Config {
    # Skip overwriting the config file if it already exists
    if (Test-Path $args) { return }

    @"
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
  # - #   type: file_input
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
"@ > $args
  }

  # This will create a new agent service.
  function New-AgentService {
    Show-Header "Creating Service"
    Add-Indent
    Install-AgentService
    Start-AgentService
    Complete
    Remove-Indent
  }

  # This will install the agent service.
  function Install-AgentService {
    Show-ColorText 'Installing ' '' "$SERVICE_NAME" DarkCyan ' service...'

    $service_params = @{
      Name           = "$SERVICE_NAME"
      DisplayName    = "$SERVICE_NAME"
      BinaryPathName = "$script:binary_location --config $script:agent_home\config.yaml --log_file $script:agent_home\$SERVICE_NAME.log --database $script:agent_home\$SERVICE_NAME.db --plugin_dir $script:plugin_dir"
      Description    = "Monitors and processes logs."
      StartupType    = "Automatic"
    }

    try {
      New-Service @service_params -ErrorAction Stop | Out-Null
    }
    catch {
      Exit-Error $MyInvocation.ScriptLineNumber "Failed to install the $SERVICE_NAME service." 'Please ensure you have permission to install services.'
    }


    $script:startup_cmd = "net start `"$SERVICE_NAME`""
    $script:shutdown_cmd = "net stop `"$SERVICE_NAME`""
    $script:autostart = "Yes"
  }

  # This will start the agent service.
  function Start-AgentService {
    Show-ColorText 'Starting service...'
    try {
      Start-Service -name $SERVICE_NAME -ErrorAction Stop
    }
    catch {
      $script:START_SERVICE = $FALSE
      Show-ColorText "Warning: An error prevented service startup: $($_.Exception.Message)" Yellow
      Show-ColorText "A restart may be required to start the service on some systems." Yellow
    }
  }

  # This will finish the install by printing out the results.
  function Complete-Install {
    Show-AgentInfo
    Show-Troubleshooting
    Show-InstallComplete
  }

  # This will display information about the agent after install.
  function Show-AgentInfo {
    Show-Header 'Information'
    Add-Indent
    Show-ColorText 'Stanza Home:   ' '' "$script:agent_home" DarkCyan
    Show-ColorText 'Stanza Config: ' '' "$script:agent_home\config.yaml"
    Show-ColorText 'Start On Boot: ' '' "$script:autostart" DarkCyan
    Show-ColorText 'Start Command: ' '' "$script:startup_cmd" DarkCyan
    Show-ColorText 'Stop Command:  ' '' "$script:shutdown_cmd" DarkCyan
    Remove-Indent
  }

  function Show-Troubleshooting {
    Show-Header 'Troubleshooting'
    Add-Indent
    Show-ColorText "To troubleshoot issues, stanza can be run manually for faster iteration."
    Show-ColorText '1) Stop the stanza service: ' '' "$script:shutdown_cmd" DarkCyan
    Show-ColorText '2) Navigate to the stanza home directory: ' '' "cd $script:agent_home" DarkCyan
    Show-ColorText '3) Run stanza manually: ' '' '.\stanza.exe --debug' DarkCyan
    Remove-Indent
  }

  # This will provide a user friendly message after the installation is complete.
  function Show-InstallComplete {
    Show-Header 'Installation Complete!' Green
    Add-Indent
    if ( $script:START_SERVICE ) {
      Show-ColorText "Your agent is installed and running.`n" Green
    }
    else {
      Show-ColorText 'Your agent is installed but not running.' Green
      Show-ColorText "Please restart to complete service installation.`n" Yellow
    }
    Remove-Indent
  }

  function Main {
    [cmdletbinding()]
    param (
      [Alias('y', 'accept_defaults')]
      [string]$script:accept_defaults,

      [Alias('v', 'version')]
      [string]$script:version,

      [Alias('i', 'install_dir')]
      [string]$script:install_dir = '',

      [Alias('u', 'service_user')]
      [string]$script:service_user,

      [Alias('h', 'help')]
      [switch]$script:help
    )
    try {
      Set-Variables
      # Variables which should be reset if the user calls Log-Agent-Install without redownloading script
      $script:indent = ''
      $script:START_SERVICE = $TRUE
      if ($MyInvocation.Line -match 'Log-Agent-Install.*') {
        $script:rerun_command = $matches[0]
      }
      if ($PSBoundParameters.ContainsKey("script:help")) {
        Show-Usage
      }
      else {
        Set-Window-Title
        Show-Separator
        Test-Prerequisites
        Set-InstallVariables
        Set-Permissions
        Remove-AgentService
        Get-StanzaBinary
        Get-StanzaPlugins
        New-AgentConfig
        New-AgentService
        Complete-Install
        Show-Separator
      }
      $exited_success = $true
    }
    catch {
      if ($_.Exception.Message) {
        if ($script:IS_PS_ISE) {
          $psIse.Options.ConsolePaneBackgroundColor = $script:DEFAULT_BG_COLOR
        }
        else {
          $host.ui.rawui.BackgroundColor = $script:DEFAULT_BG_COLOR
        }

        Show-ColorText "$_" Red
      }
      $exited_error = $true
    }
    finally {
      Restore-Window-Title
      if (!$exited_success -and !$exited_error) {
        # Write-Host is required here; anything that uses the pipeline will block
        # and not print if the user cancels with ctrl+c.
        Write-Host "`nScript canceled by user.`n" -ForegroundColor Yellow
      }
    }
  }

  set-alias 'Log-Agent-Install' -value Main
  Export-ModuleMember -Function 'Main', 'Get-ProjectMetadata' -alias 'Log-Agent-Install'
} | Out-Null
