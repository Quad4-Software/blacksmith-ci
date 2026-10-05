# Bootstrap for an autoscaled Windows host running Crow's hyperv backend.
#
# Pass it to the autoscaler with CROW_PROVIDER_USERDATA_FILE, together with
# CROW_OS_TYPE=windows. It is rendered as a Go template, which fills in the
# agent zip URL from CROW_AGENT_IMAGE and the agent environment, including
# everything in CROW_AGENT_ENV. Template syntax is kept out of the comments,
# where it would be rendered too.
#
# It turns a stock Windows Server 2022 VM into a hyperv backend host:
#
#   1. Downloads the agent and this backend's helper scripts.
#   2. Installs the Hyper-V role, and reboots to activate it.
#   3. On every boot, ensures a NAT switch with DHCP for the guests, builds the
#      golden image with git in it if there is none yet, and starts the agent.
#
# The golden image is built on the host, so a fresh host takes about 15
# minutes before its agent connects: allow for that in the autoscaler's
# CROW_AGENT_INACTIVITY_TIMEOUT, which defaults to 30 minutes.
#
# Settings, through CROW_AGENT_ENV; all optional:
#
#   CROW_BACKEND_HYPERV_*         Any backend setting. The defaults below fill
#                                 in what is not set.
#   CROW_HYPERV_SCRIPTS_URL       Where to download the scripts next to this
#                                 one from. Defaults to the Crow repository at
#                                 the tag of the agent's own release, and is
#                                 required for an agent that is not a release
#                                 build: nothing is fetched from a moving
#                                 branch, since it runs as SYSTEM.
#   CROW_HYPERV_GUEST_DNS         DNS servers handed to guests by DHCP,
#                                 comma-separated. Defaults to 1.1.1.1,8.8.8.8.
#
# The guest password is generated on the host when
# CROW_BACKEND_HYPERV_GUEST_PASSWORD is not set. The golden image is built on
# the same host with the same password, so nothing else needs to know it.
#
# Never reboot inside this script. On Azure it runs as a CustomScriptExtension,
# which reports a reboot part-way through as a failure, and the autoscaler
# then deletes the VM. The reboot is scheduled to happen after it has exited.

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$agentDir = 'C:\crow-agent'
foreach ($dir in @($agentDir, 'C:\crow-images', 'C:\crow-vms')) {
  New-Item -ItemType Directory -Force -Path $dir | Out-Null
}

# Prefer IPv4 in getaddrinfo. When a host publishes AAAA records on an
# IPv4-only network, Windows tries IPv6 first and blocks for about 30 seconds
# per attempt.
netsh interface ipv6 set prefixpolicy ::ffff:0.0.0.0/96 100 4 | Out-Null

function Get-File([string]$Url, [string]$Destination) {
  & curl.exe --fail --location --silent --show-error --retry 3 --output $Destination $Url
  if ($LASTEXITCODE -ne 0) { throw "downloading $Url failed with curl exit code $LASTEXITCODE" }
}

########################################################################
# Agent and its environment
########################################################################

$agentExe = Join-Path $agentDir 'crow-agent.exe'
if (-not (Test-Path -LiteralPath $agentExe)) {
  $zip = Join-Path $env:TEMP 'crow-agent.zip'
  Get-File '{{ .Image }}' $zip
  Expand-Archive -Path $zip -DestinationPath $agentDir -Force
  Remove-Item -LiteralPath $zip -Force
}

{{- range $key, $value := .Environment }}
[Environment]::SetEnvironmentVariable('{{ $key }}', '{{ $value }}', 'Machine')
{{- end }}

function Set-MachineDefault([string]$Name, [string]$Value) {
  if (-not [Environment]::GetEnvironmentVariable($Name, 'Machine')) {
    [Environment]::SetEnvironmentVariable($Name, $Value, 'Machine')
  }
}

Set-MachineDefault 'CROW_BACKEND' 'hyperv'
Set-MachineDefault 'CROW_BACKEND_HYPERV_BASE_VHDX' 'C:\crow-images\golden.vhdx'
Set-MachineDefault 'CROW_BACKEND_HYPERV_VM_PATH' 'C:\crow-vms'
Set-MachineDefault 'CROW_BACKEND_HYPERV_GUEST_USERNAME' 'crow'
Set-MachineDefault 'CROW_BACKEND_HYPERV_SWITCH' 'crow-nat'
Set-MachineDefault 'CROW_HYPERV_GUEST_DNS' '1.1.1.1,8.8.8.8'

if (-not [Environment]::GetEnvironmentVariable('CROW_BACKEND_HYPERV_GUEST_PASSWORD', 'Machine')) {
  # One character from each class Windows scores, then random ones from all of
  # them: 32 characters comfortably meet the complexity policy.
  $classes = @('abcdefghijkmnopqrstuvwxyz', 'ABCDEFGHJKLMNPQRSTUVWXYZ', '23456789', '-_.+=')
  $rng = [Security.Cryptography.RandomNumberGenerator]::Create()
  $pick = {
    param([string]$Set)
    $bytes = New-Object byte[] 4
    $rng.GetBytes($bytes)
    $Set[[BitConverter]::ToUInt32($bytes, 0) % $Set.Length]
  }
  $chars = @($classes | ForEach-Object { & $pick $_ })
  $all = -join $classes
  while ($chars.Count -lt 32) { $chars += & $pick $all }
  # The class-ordered start is a pattern worth removing; the shuffle itself
  # carries no secret, so Get-Random is fine for it.
  $password = -join ($chars | Sort-Object { Get-Random })
  [Environment]::SetEnvironmentVariable('CROW_BACKEND_HYPERV_GUEST_PASSWORD', $password, 'Machine')
}

$scriptsUrl = [Environment]::GetEnvironmentVariable('CROW_HYPERV_SCRIPTS_URL', 'Machine')
if (-not $scriptsUrl) {
  # The scripts that shipped with the agent being installed. A release prints
  # "crow-agent version 6.10.0" and is tagged v6.10.0.
  $agentVersion = ((& $agentExe --version) -join ' ') -replace '^.*version\s+', ''
  if ($agentVersion -notmatch '^\d+\.\d+\.\d+$') {
    throw "agent version '$agentVersion' is not a release, so there is no tag to fetch the hyperv scripts from; set CROW_HYPERV_SCRIPTS_URL"
  }
  $scriptsUrl = "https://codefloe.com/crowci/crow/raw/tag/v$agentVersion/contrib/hyperv"
  [Environment]::SetEnvironmentVariable('CROW_HYPERV_SCRIPTS_URL', $scriptsUrl, 'Machine')
}
$scriptsUrl = $scriptsUrl.TrimEnd('/')
foreach ($name in @('build-golden-image.ps1', 'guest-setup-git.ps1', 'setup-nat-network.ps1')) {
  Get-File "$scriptsUrl/$name" (Join-Path $agentDir $name)
}

########################################################################
# Start script, run at every boot
########################################################################

# Network and golden image both need the Hyper-V role active, which it is only
# after the reboot, so they happen here rather than above. Running at every
# boot, and skipping what already exists, also repairs a host whose image
# build was interrupted.
$startScript = @'
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
Start-Transcript -Path 'C:\crow-agent\start.log' -Append | Out-Null

# The Task Scheduler may hand this process an environment from before the
# bootstrap set the agent's, so read it from the registry.
foreach ($entry in [Environment]::GetEnvironmentVariables('Machine').GetEnumerator()) {
  Set-Item -Path "Env:$($entry.Key)" -Value $entry.Value
}

# Fail closed. This host exists to run workflows inside VMs; an agent on it
# with any other backend, or with the backend left to auto-detection, would
# run them on the host itself.
if ($env:CROW_BACKEND -ne 'hyperv') {
  throw "CROW_BACKEND is '$env:CROW_BACKEND', not 'hyperv'; refusing to start the agent"
}

$switch = $env:CROW_BACKEND_HYPERV_SWITCH
# Only the default switch is managed here. One with any other name is the
# operator's, set up in a custom image or by their own script.
if ($switch -eq 'crow-nat') {
  & 'C:\crow-agent\setup-nat-network.ps1' -SwitchName $switch -DnsServer ($env:CROW_HYPERV_GUEST_DNS -split ',')
}

if (-not (Test-Path -LiteralPath $env:CROW_BACKEND_HYPERV_BASE_VHDX)) {
  & 'C:\crow-agent\build-golden-image.ps1' -Path $env:CROW_BACKEND_HYPERV_BASE_VHDX `
    -Username $env:CROW_BACKEND_HYPERV_GUEST_USERNAME `
    -Password (ConvertTo-SecureString $env:CROW_BACKEND_HYPERV_GUEST_PASSWORD -AsPlainText -Force) `
    -SwitchName $switch -SetupScript 'C:\crow-agent\guest-setup-git.ps1'
}

Stop-Transcript | Out-Null
& 'C:\crow-agent\crow-agent.exe'
exit $LASTEXITCODE
'@
Set-Content -LiteralPath (Join-Path $agentDir 'start.ps1') -Value $startScript -Encoding UTF8

########################################################################
# Hyper-V
########################################################################

$needsReboot = $false
if (-not (Get-WindowsFeature -Name Hyper-V).Installed) {
  # No -Restart: see the note at the top.
  $result = Install-WindowsFeature -Name Hyper-V -IncludeManagementTools
  # RestartNeeded is No, Yes or Maybe; anything but No gets one.
  $needsReboot = $result.RestartNeeded -ne 'No'
}

########################################################################
# Agent task
########################################################################

# A scheduled task at startup rather than a Windows service: the agent does
# not implement the service control protocol, so the service manager would
# kill it after its 30-second start timeout.
if (-not (Get-ScheduledTask -TaskName 'crow-agent' -ErrorAction SilentlyContinue)) {
  $action = New-ScheduledTaskAction -Execute 'powershell.exe' `
    -Argument '-NoProfile -NonInteractive -ExecutionPolicy Bypass -File C:\crow-agent\start.ps1'
  $trigger = New-ScheduledTaskTrigger -AtStartup
  $principal = New-ScheduledTaskPrincipal -UserId 'SYSTEM' -LogonType ServiceAccount -RunLevel Highest
  $settings = New-ScheduledTaskSettingsSet -RestartCount 5 -RestartInterval (New-TimeSpan -Minutes 1) `
    -ExecutionTimeLimit ([TimeSpan]::Zero) -DontStopOnIdleEnd
  Register-ScheduledTask -TaskName 'crow-agent' -Action $action -Trigger $trigger `
    -Principal $principal -Settings $settings | Out-Null
}

if ($needsReboot) {
  # The task starts the agent after the reboot. The delay has to outlast this
  # script, or the extension running it is reported as failed.
  Write-Output 'roles installed; rebooting in 120 seconds to activate them'
  shutdown.exe /r /t 120 /c 'crow: activating Hyper-V'
} else {
  Start-ScheduledTask -TaskName 'crow-agent'
}
