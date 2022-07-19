files=(
    bin/otelcontribcol_darwin_arm64
    bin/otelcontribcol_darwin_amd64
    bin/otelcontribcol_linux_arm64
    bin/otelcontribcol_linux_ppc64le
    bin/otelcontribcol_linux_amd64
    bin/otelcontribcol_windows_amd64.exe
    dist/otel-contrib-collector-*.aarch64.rpm
    dist/otel-contrib-collector_*_amd64.deb
    dist/otel-contrib-collector-*.x86_64.rpm
    dist/otel-contrib-collector_*_arm64.deb
    dist/otel-contrib-collector_*_ppc64le.deb
    dist/otel-contrib-collector_*_ppc64le.rpm
    dist/otel-contrib-collector-*amd64.msi

);
for f in "${files[@]}"
do
    if [[ ! -f $f ]]
    then
        echo "$f does not exist."
        echo "::set-output name=passed::false"
        exit 0
    fi
done
echo "::set-output name=passed::true"
