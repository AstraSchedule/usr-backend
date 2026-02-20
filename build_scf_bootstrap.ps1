$Env:GOOS="linux"
$Env:GOARCH="amd64"
go mod tidy
if (test-path .\scf_bootstrap)
{
    Remove-Item .\scf_bootstrap -Force
}
go build -o scf_bootstrap
wsl chmod +x scf_bootstrap
if (test-path .\scf.zip)
{
    Remove-Item .\scf.zip -Force
}
wsl 7z a -tzip "scf.zip" "scf_bootstrap" "config.toml" -mx0
if (test-path .\scf_bootstrap)
{
    Remove-Item .\scf_bootstrap -Force
}