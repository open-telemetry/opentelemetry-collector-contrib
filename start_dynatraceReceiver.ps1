# .env-Datei laden und Umgebungsvariablen setzen
Get-Content .env | ForEach-Object {
    $name, $value = $_ -split '=', 2
    [System.Environment]::SetEnvironmentVariable($name, $value)
}

# OpenTelemetry Collector starten
.\bin\otelcontribcol_windows_amd64.exe --config=receiver/dynatracereceiver/config.yaml
