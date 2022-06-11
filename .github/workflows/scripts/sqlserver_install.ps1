function Install-SQLServer2019 {
    Write-Host "Downloading SQL Server 2019..."
    $Path = $env:TEMP
    $Installer = "SQL2019-SSEI-Dev.exe"
    $URL = "https://go.microsoft.com/fwlink/?linkid=866662"
    Invoke-WebRequest $URL -OutFile $Path\$Installer

    Write-Host "Installing SQL Server..."
    Start-Process -FilePath $Path\$Installer -Args "/ACTION=INSTALL /IACCEPTSQLSERVERLICENSETERMS /QUIET" -Verb RunAs -Wait
    Remove-Item $Path\$Installer
}

Install-SQLServer2019
