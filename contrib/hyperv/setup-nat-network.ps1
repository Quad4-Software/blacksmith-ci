<#
.SYNOPSIS
  Creates a NAT switch with DHCP for the guests of Crow's hyperv backend.

.DESCRIPTION
  Workflow VMs attached to this switch can reach the internet through the
  host, and nothing outside the host can reach them. Point
  CROW_BACKEND_HYPERV_SWITCH at it.

  Hyper-V's NAT forwards traffic but does not hand out addresses, so a guest on
  a bare NAT switch never gets one. This also installs the DHCP Server role and
  gives it a scope on the switch's subnet. The DHCP server only listens on
  interfaces with a static address, which the switch's host side is and a
  cloud VM's own adapter is not, so it does not answer on the host's network.

  It also blocks guests from every service on the host itself except DHCP,
  through the host firewall. Keeping guests off other networks, such as the
  cloud's metadata service, is the backend's job, through
  CROW_BACKEND_HYPERV_BLOCKED_NETWORKS.

  Run it on the Hyper-V host as an administrator. It is idempotent: anything
  that already exists is left alone, so it is safe to run at every boot.

.PARAMETER SwitchName
  Name of the internal switch, and of the NAT and DHCP scope that go with it.

.PARAMETER Subnet
  The first three octets of the /24 the guests get addresses in. The host
  takes .1, and guests are leased .100 to .250. It must not overlap any
  network the host is on.

.PARAMETER DnsServer
  DNS servers handed to guests.

.EXAMPLE
  .\setup-nat-network.ps1 -SwitchName crow-nat
#>
[CmdletBinding()]
param(
  [string]$SwitchName = 'crow-nat',

  [string]$Subnet = '192.168.176',

  [string[]]$DnsServer = @('1.1.1.1', '8.8.8.8')
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$hostAddress = "$Subnet.1"
$scopeId = "$Subnet.0"
$alias = "vEthernet ($SwitchName)"

if (-not (Get-VMSwitch -Name $SwitchName -ErrorAction SilentlyContinue)) {
  # Internal: no uplink to a physical adapter, so the only way out is the NAT.
  New-VMSwitch -SwitchType Internal -Name $SwitchName | Out-Null
}
if (-not (Get-NetIPAddress -InterfaceAlias $alias -IPAddress $hostAddress -ErrorAction SilentlyContinue)) {
  New-NetIPAddress -InterfaceAlias $alias -IPAddress $hostAddress -PrefixLength 24 | Out-Null
}
if (-not (Get-NetNat -Name $SwitchName -ErrorAction SilentlyContinue)) {
  New-NetNat -Name $SwitchName -InternalIPInterfaceAddressPrefix "$scopeId/24" | Out-Null
}

if (-not (Get-WindowsFeature -Name DHCP).Installed) {
  Install-WindowsFeature -Name DHCP -IncludeManagementTools | Out-Null
}
if (-not (Get-DhcpServerv4Scope -ScopeId $scopeId -ErrorAction SilentlyContinue)) {
  # Four-hour leases: a workflow VM lives for one workflow, so a short lease
  # returns its address to the pool soon after the VM is gone.
  Add-DhcpServerv4Scope -Name $SwitchName -StartRange "$Subnet.100" -EndRange "$Subnet.250" `
    -SubnetMask 255.255.255.0 -LeaseDuration 0.04:00:00
  # -Force skips probing the DNS servers, which may not answer yet this early
  # in a boot.
  Set-DhcpServerv4OptionValue -ScopeId $scopeId -Router $hostAddress -DnsServer $DnsServer -Force
  # The service picks up the new interface binding only on a restart.
  Restart-Service -Name DHCPServer
}

# Guests reach the host at the gateway address, and Windows Server listens on
# RDP, WinRM, SMB and RPC there out of the box. Block everything a guest could
# open on the host except DHCP, which is all the host serves them. Block rules
# take precedence over the allow rules those services ship with. Traffic the
# NAT forwards for a guest is not addressed to the host, so egress is
# unaffected.
$ruleGroup = "crow: $SwitchName guests"
if (-not (Get-NetFirewallRule -Group $ruleGroup -ErrorAction SilentlyContinue)) {
  New-NetFirewallRule -Group $ruleGroup -DisplayName "$ruleGroup (TCP)" -Direction Inbound -Action Block `
    -InterfaceAlias $alias -Protocol TCP | Out-Null
  New-NetFirewallRule -Group $ruleGroup -DisplayName "$ruleGroup (UDP)" -Direction Inbound -Action Block `
    -InterfaceAlias $alias -Protocol UDP -LocalPort '1-66', '68-65535' | Out-Null
}

Write-Output "guests on switch '$SwitchName' get addresses in $scopeId/24 through $hostAddress"
