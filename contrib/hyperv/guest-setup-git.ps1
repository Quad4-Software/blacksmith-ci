<#
.SYNOPSIS
  Installs Git for Windows into a golden image for Crow's hyperv backend.

.DESCRIPTION
  Pass it to build-golden-image.ps1 with -SetupScript, together with a
  -SwitchName that gives the build VM network access:

    .\build-golden-image.ps1 -Path C:\crow-images\golden.vhdx -Username crow `
      -Password $password -SwitchName crow-nat -SetupScript .\guest-setup-git.ps1

  The clone step runs git inside the guest, so an image without it can only
  run workflows that skip the clone.

  It runs inside the guest, as the guest administrator, over PowerShell
  Direct. Use it as a template for installing the rest of your toolchain.
#>

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

# renovate: datasource=github-releases depName=git-for-windows/git extractVersion=^v(?<version>\d+\.\d+\.\d+)\.windows\.\d+$
$gitVersion = '2.50.0'

# Prefer IPv4 in getaddrinfo. A workflow VM sits behind the host's IPv4 NAT,
# and when a host publishes AAAA records Windows tries IPv6 first and blocks
# for about 30 seconds per attempt, which on a clone looks like a hang. This
# persists in the image, so every workflow VM inherits it.
netsh interface ipv6 set prefixpolicy ::ffff:0.0.0.0/96 100 4 | Out-Null

$installer = Join-Path $env:TEMP 'git-installer.exe'
& curl.exe --fail --location --silent --show-error --retry 3 --output $installer `
  "https://github.com/git-for-windows/git/releases/download/v$gitVersion.windows.1/Git-$gitVersion-64-bit.exe"
if ($LASTEXITCODE -ne 0) { throw "downloading Git for Windows failed with curl exit code $LASTEXITCODE" }

$process = Start-Process -FilePath $installer -Wait -PassThru `
  -ArgumentList '/VERYSILENT', '/NORESTART', '/NOCANCEL', '/SP-', '/SUPPRESSMSGBOXES'
if ($process.ExitCode -ne 0) { throw "the Git for Windows installer exited with $($process.ExitCode)" }
Remove-Item -LiteralPath $installer -Force

$gitCmd = 'C:\Program Files\Git\cmd'
$machinePath = [Environment]::GetEnvironmentVariable('PATH', 'Machine')
if ($machinePath -notlike "*$gitCmd*") {
  [Environment]::SetEnvironmentVariable('PATH', "$machinePath;$gitCmd", 'Machine')
}

# Windows schannel checks certificate revocation on every TLS handshake, and a
# slow revocation endpoint stalls each one for 30 to 60 seconds. --system,
# because the clone step overrides HOME, so no per-user config is read.
& (Join-Path $gitCmd 'git.exe') config --system http.schannelCheckRevoke false
if ($LASTEXITCODE -ne 0) { throw "git config failed with exit code $LASTEXITCODE" }

& (Join-Path $gitCmd 'git.exe') --version
