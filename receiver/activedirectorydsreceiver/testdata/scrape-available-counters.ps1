# This powershell script finds all DirectoryServices counter paths and dumps them to counters.txt.
# This should be run on a system with the AD-Domain-Services Windows feature enabled.
(Get-Counter -ListSet DirectoryServices).Paths | Set-Content -Path "$PSScriptRoot\counters.txt"
