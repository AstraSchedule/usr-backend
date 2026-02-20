go mod tidy
rm -f scf_bootstrap
go build -o scf_bootstrap
chmod +x scf_bootstrap
rm -f scf.zip
7z a -tzip "scf.zip" "scf_bootstrap" "config.toml" -mx0
rm -f scf_bootstrap