<#
.SYNOPSIS
  Builds a golden VHDX for Crow's hyperv backend from a Windows Server ISO.

.DESCRIPTION
  Every workflow on the hyperv backend boots a differencing child of one
  base VHDX, so that disk has to exist before the agent can run anything.
  This builds one without Packer or any download beyond the ISO:

    1. applies the chosen edition from the ISO onto a fresh UEFI-partitioned
      VHDX with DISM,
    2. adds an answer file that creates the administrator account the agent
      logs in with and skips every first-boot prompt,
    3. boots it once in a throwaway VM, so that first-boot setup has already
      run and each workflow VM starts in seconds rather than minutes, and
    4. shuts it down and compacts it.

  The image is not generalized with sysprep. Every workflow VM is a disposable
  clone with no network adapter by default, so sharing a machine SID and
  computer name does not matter, and skipping sysprep is what spares each
  clone the specialize pass on boot.

  The image is built under a temporary name and only moved into place once
  it is complete, so a failed or interrupted run never leaves a half-built
  disk where the agent would pick it up.

  Run it on the Hyper-V host, as an administrator. It needs about 20 GB free
  for the ISO and the disk. On a 4-vCPU Azure D4s_v5 it takes about 7
  minutes: under 2 to download the ISO, 4 to apply the image and 1 for the
  first boot.

.PARAMETER Path
  Where to write the golden VHDX. Point CROW_BACKEND_HYPERV_BASE_VHDX at it.

.PARAMETER Username
  The local administrator account to create. Give the agent the same name in
  CROW_BACKEND_HYPERV_GUEST_USERNAME. "Administrator" uses the built-in
  account instead of creating one.

.PARAMETER Password
  That account's password, which goes into CROW_BACKEND_HYPERV_GUEST_PASSWORD.
  It has to meet the Windows complexity rules: at least 8 characters, from
  three of upper case, lower case, digits and symbols.

.PARAMETER IsoPath
  A Windows Server ISO already on disk. When omitted, the ISO is downloaded
  from IsoUrl and deleted afterwards.

.PARAMETER IsoUrl
  Where to download the ISO from. Defaults to the Windows Server 2022
  evaluation, whose license expires 180 days after the image is built: an
  expired guest shuts itself down every hour, so rebuild before then, or use
  your own licensed media.

.PARAMETER ImageName
  The edition to apply, as listed by Get-WindowsImage. Defaults to Server Core,
  which is a third the size of the Desktop Experience and boots faster.

.PARAMETER SizeBytes
  The maximum size of the dynamically expanding disk. Workflow VMs write to
  their differencing child, so this caps what the guest's C: can grow to.

.EXAMPLE
  .\build-golden-image.ps1 -Path C:\crow-images\ws2022-golden.vhdx -Username crow -Password (Read-Host -AsSecureString)
#>
[CmdletBinding()]
param(
  [Parameter(Mandatory = $true)]
  [string]$Path,

  [Parameter(Mandatory = $true)]
  [string]$Username,

  [Parameter(Mandatory = $true)]
  [SecureString]$Password,

  [string]$IsoPath,

  [string]$IsoUrl = 'https://go.microsoft.com/fwlink/p/?LinkID=2195280&clcid=0x409&culture=en-us&country=US',

  [string]$ImageName = 'Windows Server 2022 Datacenter Evaluation',

  [UInt64]$SizeBytes = 64GB
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

# How long the first boot may take before the build gives up. It is usually
# 5 to 10 minutes; specialize and OOBE on a slow disk can take several times
# that.
$firstBootTimeout = [TimeSpan]::FromMinutes(45)

# The well-known GPT partition type of an EFI system partition.
$espGptType = '{c12a7328-f81f-11d2-ba4b-00a0c93ec93b}'

function Write-Step([string]$Message) {
  Write-Output ("[{0:HH:mm:ss}] {1}" -f (Get-Date), $Message)
}

function ConvertTo-PlainText([SecureString]$Secure) {
  $ptr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($Secure)
  try {
    return [Runtime.InteropServices.Marshal]::PtrToStringBSTR($ptr)
  } finally {
    [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($ptr)
  }
}

# The answer file Windows reads from \Windows\Panther on its first boot. The
# password is in plain text here, which is why the file is deleted from the
# guest once setup has consumed it.
function New-UnattendXml([string]$User, [string]$PlainPassword) {
  $xmlUser = [Security.SecurityElement]::Escape($User)
  $xmlPassword = [Security.SecurityElement]::Escape($PlainPassword)
  $component = 'processorArchitecture="amd64" publicKeyToken="31bf3856ad364e35" language="neutral" versionScope="nonSxS"'

  # The built-in Administrator only needs its password set. Any other name is
  # created as an additional member of Administrators; the built-in account
  # still gets the same password, because Windows otherwise demands one
  # interactively on the console and setup never finishes.
  $localAccounts = ''
  if ($User -ne 'Administrator') {
    $localAccounts = @"
        <LocalAccounts>
          <LocalAccount wcm:action="add">
            <Name>$xmlUser</Name>
            <Group>Administrators</Group>
            <Password>
              <Value>$xmlPassword</Value>
              <PlainText>true</PlainText>
            </Password>
          </LocalAccount>
        </LocalAccounts>
"@
  }

  return @"
<?xml version="1.0" encoding="utf-8"?>
<unattend xmlns="urn:schemas-microsoft-com:unattend" xmlns:wcm="http://schemas.microsoft.com/WMIConfig/2002/State">
  <settings pass="specialize">
    <component name="Microsoft-Windows-Shell-Setup" $component>
      <ComputerName>crow-golden</ComputerName>
      <TimeZone>UTC</TimeZone>
    </component>
  </settings>
  <settings pass="oobeSystem">
    <component name="Microsoft-Windows-International-Core" $component>
      <InputLocale>en-US</InputLocale>
      <SystemLocale>en-US</SystemLocale>
      <UILanguage>en-US</UILanguage>
      <UserLocale>en-US</UserLocale>
    </component>
    <component name="Microsoft-Windows-Shell-Setup" $component>
      <OOBE>
        <HideEULAPage>true</HideEULAPage>
        <SkipMachineOOBE>true</SkipMachineOOBE>
        <SkipUserOOBE>true</SkipUserOOBE>
        <ProtectYourPC>3</ProtectYourPC>
      </OOBE>
      <UserAccounts>
        <AdministratorPassword>
          <Value>$xmlPassword</Value>
          <PlainText>true</PlainText>
        </AdministratorPassword>
$localAccounts
      </UserAccounts>
    </component>
  </settings>
</unattend>
"@
}

$principal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
  throw 'Run this as an administrator: it partitions a disk and creates a VM.'
}
if (-not (Get-Command New-VM -ErrorAction SilentlyContinue)) {
  throw 'The Hyper-V PowerShell module is missing. Install the Hyper-V role first.'
}
if (Test-Path -LiteralPath $Path) {
  throw "$Path already exists. Delete it first to rebuild; workflow VMs must not be running from it."
}

$plainPassword = ConvertTo-PlainText $Password
$credential = New-Object Management.Automation.PSCredential($Username, $Password)

$Path = [IO.Path]::GetFullPath($Path)
# New-VHD refuses any name not ending in .vhd or .vhdx, so the marker goes
# before the extension.
$partial = [IO.Path]::ChangeExtension($Path, '.partial' + [IO.Path]::GetExtension($Path))
$suffix = -join ((48..57) + (97..122) | Get-Random -Count 6 | ForEach-Object { [char]$_ })
$vmName = "crow-golden-build-$suffix"

$downloadedIso = $null
$mountedIso = $null
$vhdMounted = $false
$built = $false

try {
  New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Path) | Out-Null
  if (Test-Path -LiteralPath $partial) {
    # Left by an earlier run that was killed; it is never valid to resume.
    Remove-Item -LiteralPath $partial -Force
  }

  if (-not $IsoPath) {
    $downloadedIso = Join-Path $env:TEMP "crow-golden-$suffix.iso"
    Write-Step "downloading $IsoUrl"
    # curl.exe ships with Windows Server 2019 and later, and unlike
    # Invoke-WebRequest it does not buffer a 5 GB download in memory.
    & curl.exe --fail --location --silent --show-error --retry 3 --output $downloadedIso $IsoUrl
    if ($LASTEXITCODE -ne 0) { throw "downloading the ISO failed with curl exit code $LASTEXITCODE" }
    $IsoPath = $downloadedIso
  }

  Write-Step "mounting $IsoPath"
  $mountedIso = Mount-DiskImage -ImagePath ([IO.Path]::GetFullPath($IsoPath)) -PassThru
  $isoLetter = ($mountedIso | Get-Volume).DriveLetter
  $wim = @("${isoLetter}:\sources\install.wim", "${isoLetter}:\sources\install.esd") |
    Where-Object { Test-Path -LiteralPath $_ } | Select-Object -First 1
  if (-not $wim) { throw "$IsoPath has no sources\install.wim or install.esd; is it a Windows Server ISO?" }

  $images = @(Get-WindowsImage -ImagePath $wim)
  $image = $images | Where-Object { $_.ImageName -eq $ImageName } | Select-Object -First 1
  if (-not $image) {
    $names = ($images | ForEach-Object { "'$($_.ImageName)'" }) -join ', '
    throw "no edition named '$ImageName' in the ISO. Available: $names"
  }

  Write-Step "creating $partial"
  New-VHD -Path $partial -SizeBytes $SizeBytes -Dynamic | Out-Null
  $disk = Mount-VHD -Path $partial -Passthru | Get-Disk
  $vhdMounted = $true
  Initialize-Disk -Number $disk.Number -PartitionStyle GPT

  # The layout Windows Setup itself creates on a UEFI machine: EFI system
  # partition, Microsoft reserved partition, then Windows.
  $esp = New-Partition -DiskNumber $disk.Number -Size 260MB -GptType $espGptType
  Format-Volume -Partition $esp -FileSystem FAT32 -NewFileSystemLabel System -Confirm:$false | Out-Null
  $esp | Add-PartitionAccessPath -AssignDriveLetter
  $espLetter = (Get-Partition -DiskNumber $disk.Number -PartitionNumber $esp.PartitionNumber).DriveLetter

  New-Partition -DiskNumber $disk.Number -Size 16MB -GptType '{e3c9e316-0b5c-4db8-817d-f92df00215ae}' | Out-Null

  $windows = New-Partition -DiskNumber $disk.Number -UseMaximumSize -AssignDriveLetter
  Format-Volume -Partition $windows -FileSystem NTFS -NewFileSystemLabel Windows -Confirm:$false | Out-Null
  $winLetter = (Get-Partition -DiskNumber $disk.Number -PartitionNumber $windows.PartitionNumber).DriveLetter

  Write-Step "applying '$ImageName'"
  Expand-WindowsImage -ImagePath $wim -Index $image.ImageIndex -ApplyPath "${winLetter}:\" | Out-Null

  & "${winLetter}:\Windows\System32\bcdboot.exe" "${winLetter}:\Windows" /s "${espLetter}:" /f UEFI
  if ($LASTEXITCODE -ne 0) { throw "bcdboot failed with exit code $LASTEXITCODE" }

  $panther = "${winLetter}:\Windows\Panther"
  New-Item -ItemType Directory -Force -Path $panther | Out-Null
  [IO.File]::WriteAllText((Join-Path $panther 'unattend.xml'), (New-UnattendXml $Username $plainPassword), (New-Object Text.UTF8Encoding($false)))

  Dismount-VHD -Path $partial
  $vhdMounted = $false
  Dismount-DiskImage -ImagePath $mountedIso.ImagePath | Out-Null
  $mountedIso = $null

  Write-Step "first boot in $vmName"
  New-VM -Name $vmName -Generation 2 -VHDPath $partial -MemoryStartupBytes 4GB | Out-Null
  Set-VMProcessor -VMName $vmName -Count 2
  # The same isolation the backend gives workflow VMs: nothing on the network
  # can reach a guest whose administrator password was just in a file.
  Get-VMNetworkAdapter -VMName $vmName | Remove-VMNetworkAdapter
  Start-VM -Name $vmName

  # PowerShell Direct answers as soon as the account exists, which can be
  # before setup has finished. The registry flags are what say it is done.
  $deadline = (Get-Date) + $firstBootTimeout
  while ($true) {
    $state = $null
    try {
      $state = Invoke-Command -VMName $vmName -Credential $credential -ErrorAction Stop -ScriptBlock {
        $setup = Get-ItemProperty -Path 'HKLM:\SYSTEM\Setup'
        "$($setup.SystemSetupInProgress)/$($setup.OOBEInProgress)"
      }
    } catch {
      # Not up yet, or the account has not been created yet.
    }
    if ($state -eq '0/0') { break }
    if ((Get-Date) -gt $deadline) {
      throw "the guest did not finish first-boot setup within $($firstBootTimeout.TotalMinutes) minutes (last state: '$state')"
    }
    Start-Sleep -Seconds 15
  }

  Write-Step 'first boot complete, cleaning up the guest'
  Invoke-Command -VMName $vmName -Credential $credential -ErrorAction Stop -ScriptBlock {
    # Setup masks the passwords in the copy it keeps, but the original is left
    # where it was put.
    foreach ($file in @('C:\Windows\Panther\unattend.xml', 'C:\Windows\Panther\Unattend\unattend.xml')) {
      Remove-Item -LiteralPath $file -Force -ErrorAction SilentlyContinue
    }
    # Remote UAC gives administrators other than the built-in one a filtered
    # token on remote logons. Turning it off means a step that needs elevation
    # never depends on which account name the image was built with.
    New-ItemProperty -Path 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Policies\System' `
      -Name LocalAccountTokenFilterPolicy -PropertyType DWord -Value 1 -Force | Out-Null
  }

  Write-Step 'shutting down'
  Stop-VM -Name $vmName -Force
  Remove-VM -Name $vmName -Force
  $vmName = $null

  Write-Step 'compacting'
  Mount-VHD -Path $partial -ReadOnly
  try {
    Optimize-VHD -Path $partial -Mode Full
  } finally {
    Dismount-VHD -Path $partial
  }

  Move-Item -LiteralPath $partial -Destination $Path
  $built = $true
  $size = (Get-Item -LiteralPath $Path).Length / 1GB
  Write-Step ("built {0} ({1:N1} GB)" -f $Path, $size)
} finally {
  if ($vmName -and (Get-VM -Name $vmName -ErrorAction SilentlyContinue)) {
    Stop-VM -Name $vmName -TurnOff -Force -ErrorAction SilentlyContinue
    Remove-VM -Name $vmName -Force -ErrorAction SilentlyContinue
  }
  if ($vhdMounted) {
    Dismount-VHD -Path $partial -ErrorAction SilentlyContinue
  }
  if ($mountedIso) {
    Dismount-DiskImage -ImagePath $mountedIso.ImagePath -ErrorAction SilentlyContinue | Out-Null
  }
  if ($downloadedIso) {
    Remove-Item -LiteralPath $downloadedIso -Force -ErrorAction SilentlyContinue
  }
  if (-not $built) {
    Remove-Item -LiteralPath $partial -Force -ErrorAction SilentlyContinue
  }
}
