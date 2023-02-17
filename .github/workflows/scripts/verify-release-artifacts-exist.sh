files=(
    bin/otelcontribcol_darwin_arm64
    bin/otelcontribcol_darwin_amd64
    bin/otelcontribcol_linux_arm64
    bin/otelcontribcol_linux_amd64
);
for f in "${files[@]}"
do
    if [[ ! -f $f ]]
    then
        echo "$f does not exist."
        echo "passed=false" >> $GITHUB_OUTPUT
        exit 0
    fi
done
echo "passed=true" >> $GITHUB_OUTPUT
