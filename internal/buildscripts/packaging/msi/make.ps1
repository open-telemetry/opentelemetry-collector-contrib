# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

<#
.SYNOPSIS
    Makefile like build commands for the Collector on Windows.
    
    Usage:   .\make.ps1 <Command> [-<Param> <Value> ...]
    Example: .\make.ps1 New-MSI -Config "./my-config.yaml" -Version "v0.0.2"
.PARAMETER Target
    Build target to run (Install-Tools, New-MSI)
#>
Param(
    [Parameter(Mandatory=$true, ValueFromRemainingArguments=$true)][string]$Target
)

$ErrorActionPreference = "Stop"

function Set-Path {
    $Env:Path += ";C:\Program Files (x86)\WiX Toolset v3.11\bin"
}

function Install-Tools {
    # disable progress bar support as this causes CircleCI to crash
    $OriginalPref = $ProgressPreference
    $ProgressPreference = "SilentlyContinue"
    Install-WindowsFeature Net-Framework-Core
    $ProgressPreference = $OriginalPref

    choco install wixtoolset -y
}

function New-MSI(
    [string]$Version="0.0.1",
    [string]$Config="./examples/demo/otel-collector-config.yaml"
) {
    Set-Path
    candle.exe -arch x64 -dVersion="$Version" -dConfig="$Config" internal/buildscripts/packaging/msi/opentelemetry-contrib-collector.wxs
    light.exe opentelemetry-contrib-collector.wixobj
    mkdir dist -ErrorAction Ignore
    Move-Item -Force opentelemetry-contrib-collector.msi dist/otel-contrib-collector-$Version-amd64.msi
}

function Confirm-MSI {
    Set-Path
    # ensure system32 is in Path so we can use executables like msiexec & sc
    $env:Path += ";C:\Windows\System32"
    $msipath = Resolve-Path "$pwd\dist\otel-contrib-collector-*-amd64.msi"

    # install msi, validate service is installed & running
    Start-Process -Wait msiexec "/i `"$msipath`" /qn"
    sc.exe query state=all | findstr "otelcontribcol" | Out-Null
    if ($LASTEXITCODE -ne 0) { Throw "otelcontribcol service failed to install" }

    # stop service
    Stop-Service otelcontribcol

    # start service
    Start-Service otelcontribcol

    # uninstall msi, validate service is uninstalled
    Start-Process -Wait msiexec "/x `"$msipath`" /qn"
    sc.exe query state=all | findstr "otelcontribcol" | Out-Null
    if ($LASTEXITCODE -ne 1) { Throw "otelcontribcol service failed to uninstall" }
}

$sb = [scriptblock]::create("$Target")
Invoke-Command -ScriptBlock $sb
exit 0
