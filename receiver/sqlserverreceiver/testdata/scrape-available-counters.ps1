# This powershell script finds all SQLServer counter paths and dumps them to counters.txt.
# This should be run on a system with the a running SQLServer.
(Get-Counter -ListSet "SQLServer:*").Paths | Set-Content -Path "$PSScriptRoot\counters.txt"
