# This powershell script finds all SQLServer counter paths and dumps them to counters.txt for a named instance.
# This should be run on a system with the a running SQLServer and named instance.
# This example uses a named instance of TEST_NAME.
(Get-Counter -ListSet "MSSQL$*").Paths| Set-Content -Path "$PSScriptRoot\counters.txt"
