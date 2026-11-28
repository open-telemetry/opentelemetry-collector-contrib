module github.com/open-telemetry/opentelemetry-collector-contrib/internal/tools

go 1.26.0

tool (
	go.opentelemetry.io/build-tools/checkapi
	go.opentelemetry.io/build-tools/checkfile
	go.opentelemetry.io/build-tools/chloggen
	go.opentelemetry.io/build-tools/crosslink
	go.opentelemetry.io/build-tools/githubgen
	go.opentelemetry.io/build-tools/issuegenerator
	go.opentelemetry.io/build-tools/multimod
	go.opentelemetry.io/collector/cmd/builder
	go.opentelemetry.io/collector/cmd/mdatagen
	go.opentelemetry.io/collector/cmd/schemagen
)

require go.uber.org/goleak v1.3.0

require (
	dario.cat/mergo v1.0.2 // indirect
	github.com/Microsoft/go-winio v0.6.4 // indirect
	github.com/ProtonMail/go-crypto v1.5.2 // indirect
	github.com/cenkalti/backoff/v7 v7.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cloudflare/circl v1.6.5 // indirect
	github.com/cyphar/filepath-securejoin v0.7.0 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/fatih/color v1.19.0 // indirect
	github.com/fsnotify/fsnotify v1.10.1 // indirect
	github.com/go-git/gcfg v1.5.1-0.20230307220236-3a3c6141e376 // indirect
	github.com/go-git/go-billy/v5 v5.9.2 // indirect
	github.com/go-git/go-git/v5 v5.19.3 // indirect
	github.com/go-json-experiment/json v1.27.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/goccy/go-json v0.12.0 // indirect
	github.com/goccy/go-yaml v1.19.2 // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/google/go-github/v93 v93.0.0 // indirect
	github.com/google/go-querystring v1.2.0 // indirect
	github.com/google/jsonschema-go v0.4.3 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.8 // indirect
	github.com/hashicorp/go-version v1.9.0 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/iancoleman/strcase v0.3.0 // indirect
	github.com/iimos/ucum v0.0.4 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/joshdk/go-junit v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kaptinlin/go-i18n v0.2.4 // indirect
	github.com/kaptinlin/jsonpointer v0.4.10 // indirect
	github.com/kaptinlin/jsonschema v0.6.10 // indirect
	github.com/kaptinlin/messageformat-go v0.4.10 // indirect
	github.com/kevinburke/ssh_config v1.6.0 // indirect
	github.com/knadh/koanf/maps v0.1.3 // indirect
	github.com/knadh/koanf/parsers/yaml v1.1.1 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.1 // indirect
	github.com/knadh/koanf/providers/env/v2 v2.0.2 // indirect
	github.com/knadh/koanf/providers/file v1.2.1 // indirect
	github.com/knadh/koanf/providers/fs v1.0.1 // indirect
	github.com/knadh/koanf/v2 v2.4.1 // indirect
	github.com/mattn/go-colorable v0.1.15 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/onsi/gomega v1.42.1 // indirect
	github.com/pelletier/go-toml/v2 v2.4.3 // indirect
	github.com/pjbgf/sha1cd v0.7.0 // indirect
	github.com/sagikazarmark/locafero v0.12.0 // indirect
	github.com/sergi/go-diff v1.4.0 // indirect
	github.com/skeema/knownhosts v1.3.3 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/spf13/cobra v1.10.2 // indirect
	github.com/spf13/pflag v1.1.0 // indirect
	github.com/spf13/viper v1.21.0 // indirect
	github.com/stretchr/testify v1.12.1 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/xanzy/ssh-agent v0.3.3 // indirect
	go.opentelemetry.io/build-tools v0.31.0 // indirect
	go.opentelemetry.io/build-tools/checkapi v0.31.0 // indirect
	go.opentelemetry.io/build-tools/checkfile v0.31.0 // indirect
	go.opentelemetry.io/build-tools/chloggen v0.31.0 // indirect
	go.opentelemetry.io/build-tools/crosslink v0.31.0 // indirect
	go.opentelemetry.io/build-tools/githubgen v0.31.0 // indirect
	go.opentelemetry.io/build-tools/issuegenerator v0.31.0 // indirect
	go.opentelemetry.io/build-tools/multimod v0.31.0 // indirect
	go.opentelemetry.io/collector/client v1.72.1-0.20261126161120-4c6c46bf6bc6 // indirect
	go.opentelemetry.io/collector/cmd/builder v0.166.1-0.20261126161120-4c6c46bf6bc6 // indirect
	go.opentelemetry.io/collector/cmd/mdatagen v0.166.1-0.20261126161120-4c6c46bf6bc6 // indirect
	go.opentelemetry.io/collector/cmd/schemagen v0.166.1-0.20261126161120-4c6c46bf6bc6 // indirect
	go.opentelemetry.io/collector/component v1.72.1-0.20261126161120-4c6c46bf6bc6 // indirect
	go.opentelemetry.io/collector/config/configoptional v1.72.1-0.20261126161120-4c6c46bf6bc6 // indirect
	go.opentelemetry.io/collector/config/configretry v1.72.1-0.20261126161120-4c6c46bf6bc6 // indirect
	go.opentelemetry.io/collector/config/configstorage v0.166.1-0.20261126161120-4c6c46bf6bc6 // indirect
	go.opentelemetry.io/collector/confmap v1.72.1-0.20261126161120-4c6c46bf6bc6 // indirect
	go.opentelemetry.io/collector/confmap/provider/fileprovider v1.72.1-0.20261126161120-4c6c46bf6bc6 // indirect
	go.opentelemetry.io/collector/consumer v1.72.1-0.20261126161120-4c6c46bf6bc6 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.166.1-0.20261126161120-4c6c46bf6bc6 // indirect
	go.opentelemetry.io/collector/exporter v1.72.1-0.20261126161120-4c6c46bf6bc6 // indirect
	go.opentelemetry.io/collector/exporter/exporterhelper v0.166.1-0.20261126161120-4c6c46bf6bc6 // indirect
	go.opentelemetry.io/collector/extension v1.72.1-0.20261126161120-4c6c46bf6bc6 // indirect
	go.opentelemetry.io/collector/extension/xextension v0.166.1-0.20261126161120-4c6c46bf6bc6 // indirect
	go.opentelemetry.io/collector/featuregate v1.72.1-0.20261126161120-4c6c46bf6bc6 // indirect
	go.opentelemetry.io/collector/internal/componentalias v0.166.1-0.20261126161120-4c6c46bf6bc6 // indirect
	go.opentelemetry.io/collector/internal/schemagen v0.166.1-0.20261126161120-4c6c46bf6bc6 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.166.1-0.20261126161120-4c6c46bf6bc6 // indirect
	go.opentelemetry.io/collector/pdata v1.72.1-0.20261126161120-4c6c46bf6bc6 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.166.1-0.20261126161120-4c6c46bf6bc6 // indirect
	go.opentelemetry.io/collector/pdata/xpdata v0.166.1-0.20261126161120-4c6c46bf6bc6 // indirect
	go.opentelemetry.io/collector/pipeline v1.72.1-0.20261126161120-4c6c46bf6bc6 // indirect
	go.opentelemetry.io/collector/pipeline/xpipeline v0.166.1-0.20261126161120-4c6c46bf6bc6 // indirect
	go.opentelemetry.io/collector/strmatch v0.0.0-20261126161120-4c6c46bf6bc6 // indirect
	go.opentelemetry.io/otel v1.47.0 // indirect
	go.opentelemetry.io/otel/metric v1.47.0 // indirect
	go.opentelemetry.io/otel/trace v1.47.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.28.0 // indirect
	go.yaml.in/yaml/v3 v3.0.6 // indirect
	golang.org/x/crypto v0.59.0 // indirect
	golang.org/x/mod v0.43.0 // indirect
	golang.org/x/net v0.63.0 // indirect
	golang.org/x/oauth2 v0.38.0 // indirect
	golang.org/x/sync v0.25.0 // indirect
	golang.org/x/sys v0.50.0 // indirect
	golang.org/x/text v0.44.0 // indirect
	golang.org/x/tools v0.53.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260928230214-8a89bd6388cc // indirect
	google.golang.org/grpc v1.85.0 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)
