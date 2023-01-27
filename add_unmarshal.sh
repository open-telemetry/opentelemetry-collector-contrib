#!/bin/zsh
rm -Rf .tmp-renames; mkdir .tmp-renames
find receiver -name config.go \
    -exec grep -L Unmarshal {} \; \
    -exec sh -c 'cat unmarshal_normal.txt >> {} && gofmt -w -s {}' \; \
    -exec grep -L "go.opentelemetry.io/collector/confmap" {} \; \
    -exec sh -c 'cd $(dirname {}) && go get -x "go.opentelemetry.io/collector/confmap" && cd -' \; \
    -exec sh -c "sed -i '/^package.*/a import (\"go.opentelemetry.io/collector/confmap\")' {} " \; \
    -exec gofmt -w -s {} \; \
    -exec goimports -w -local "github.com/open-telemetry/opentelemetry-collector-contrib" {} \;  \
    -exec ./.tools/gci write {} --skip-generated -s standard,default,prefix\(github.com/open-telemetry/opentelemetry-collector-contrib\) \;


# this command
grep -E -h -o "func \(.*(\*)?Config\)" ./**/config.go  | cat | sort | uniq -c
# returns
echo "still need to fix these"
find receiver -name config.go -exec grep -l -E 'func \(rCfg (\*)?Config\)' {} \; -exec sh -c "sed -i 's/cfg/rCfg/g' {}" \; -print
find receiver -name config.go -exec grep -l -E 'func \(c (\*)?Config\)' {} \; -exec sh -c "sed -i 's/cfg/c/g' {}" \; -print
find receiver -name config.go -exec grep -l -E 'func \(config (\*)?(Group)?Config\)' {} \; -exec sh -c "sed -i 's/cfg/config/g' {}" \; -print
#
# From here manually edit docker and prometheus to have cfg and config or whatever in their config.go files

# current state is jmx failing unit test lookup at a nil blah blah not sure how real it is


# -exec ./.tools/gci --skip-generated -s standard,default,prefix\(github.com/open-telemetry/opentelemetry-collector-contrib\) {} \;
rm -Rf .tmp-renames;

