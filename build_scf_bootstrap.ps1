$Env:GOOS="linux"
$Env:GOARCH="amd64"
go mod tidy
if (test-path .\bootstrap)
{
    Remove-Item .\bootstrap -Force
}
go build -o bootstrap
wsl chmod +x bootstrap
if (test-path .\scf.zip)
{
    Remove-Item .\scf.zip -Force
}
wsl 7z a -tzip "scf.zip" "bootstrap" -mx0
if (test-path .\bootstrap)
{
    Remove-Item .\bootstrap -Force
}