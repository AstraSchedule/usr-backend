$Env:GOOS="linux"
$Env:GOARCH="amd64"
go mod tidy
if (test-path .\scf_bootstrap)
{
    Remove-Item .\scf_bootstrap -Force
}
go build -o scf_bootstrap
if (test-path .\scf.zip)
{
    Remove-Item .\scf.zip -Force
}
$compress = @{
    Path = ".\scf_bootstrap", ".\config.toml"
    CompressionLevel = "Fastest"
    DestinationPath = ".\scf.Zip"
}
Compress-Archive @compress
if (test-path .\scf_bootstrap)
{
    Remove-Item .\scf_bootstrap -Force
}
