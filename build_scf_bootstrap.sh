go mod tidy
rm -f bootstrap
go build -o bootstrap
chmod +x bootstrap
rm -f scf.zip
7z a -tzip "scf.zip" "bootstrap" "config.toml" -mx0
rm -f bootstrap