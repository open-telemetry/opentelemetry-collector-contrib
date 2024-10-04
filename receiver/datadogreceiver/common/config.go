// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0 language governing permissions and
// limitations under the License.

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver"

import (
	"time"

	"github.com/DataDog/datadog-agent/pkg/obfuscate"
	"go.opentelemetry.io/collector/config/confighttp"
)

func NewConfig() *Config {
	return &Config{
		ServerConfig: confighttp.ServerConfig{
			Endpoint: "localhost:8126",
		},
		ReadTimeout: 60 * time.Second,
		Traces: TracesConfig{
			Obfuscation: ObfuscationConfig{},
		},
	}
}

type Config struct {
	confighttp.ServerConfig `mapstructure:",squash"`
	// ReadTimeout of the http server
	ReadTimeout time.Duration `mapstructure:"read_timeout"`

	// Traces holds tracing-related configurations
	Traces TracesConfig `mapstructure:"traces"`
}

// TracesConfig holds the configuration for the Datadog receiver's trace processor.
type TracesConfig struct {
	// Obfuscation holds sensitive data obufscator's configuration.
	Obfuscation ObfuscationConfig `mapstructure:"obfuscation"`
}

// ObfuscationConfig holds the configuration for obfuscating sensitive data
// for various span types.
type ObfuscationConfig struct {
	Enabled bool `mapstructure:"enabled"`

	// SQL holds the obfuscation configuration for SQL queries.
	SQL SQLConfig `mapstructure:"sql"`

	// ES holds the obfuscation configuration for ElasticSearch bodies.
	ES obfuscate.JSONConfig `mapstructure:"elasticsearch"`

	// OpenSearch holds the obfuscation configuration for OpenSearch bodies.
	OpenSearch obfuscate.JSONConfig `mapstructure:"opensearch"`

	// Mongo holds the obfuscation configuration for MongoDB queries.
	Mongo obfuscate.JSONConfig `mapstructure:"mongodb"`

	// SQLExecPlan holds the obfuscation configuration for SQL Exec Plans. This is strictly for safety related obfuscation,
	// not normalization. Normalization of exec plans is configured in SQLExecPlanNormalize.
	SQLExecPlan obfuscate.JSONConfig `mapstructure:"sql_exec_plan"`

	// SQLExecPlanNormalize holds the normalization configuration for SQL Exec Plans.
	SQLExecPlanNormalize obfuscate.JSONConfig `mapstructure:"sql_exec_plan_normalize"`

	// HTTP holds the obfuscation settings for HTTP URLs.
	HTTP obfuscate.HTTPConfig `mapstructure:"http"`

	// RemoveStackTraces specifies whether stack traces should be removed.
	// More specifically "error.stack" tag values will be cleared.
	RemoveStackTraces bool `mapstructure:"remove_stack_traces"`

	// Redis holds the configuration for obfuscating the "redis.raw_command" tag
	// for spans of type "redis".
	Redis obfuscate.RedisConfig `mapstructure:"redis"`

	// Memcached holds the configuration for obfuscating the "memcached.command" tag
	// for spans of type "memcached".
	Memcached obfuscate.MemcachedConfig `mapstructure:"memcached"`

	// CreditCards holds the configuration for obfuscating credit cards.
	CreditCards obfuscate.CreditCardsConfig `mapstructure:"credit_cards"`
}

// SQLConfig holds the config for obfuscating SQL.
type SQLConfig struct {
	// DBMS identifies the type of database management system (e.g. MySQL, Postgres, and SQL Server).
	// Valid values for this can be found at https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/database.md#connection-level-attributes
	DBMS string `mapstructure:"dbms"`

	// TableNames specifies whether the obfuscator should also extract the table names that a query addresses,
	// in addition to obfuscating.
	TableNames bool `mapstructure:"table_names"`

	// CollectCommands specifies whether the obfuscator should extract and return commands as SQL metadata when obfuscating.
	CollectCommands bool `mapstructure:"collect_commands"`

	// CollectComments specifies whether the obfuscator should extract and return comments as SQL metadata when obfuscating.
	CollectComments bool `mapstructure:"collect_comments"`

	// CollectProcedures specifies whether the obfuscator should extract and return procedure names as SQL metadata when obfuscating.
	CollectProcedures bool `mapstructure:"collect_procedures"`

	// ReplaceDigits specifies whether digits in table names and identifiers should be obfuscated.
	ReplaceDigits bool `mapstructure:"replace_digits"`

	// KeepSQLAlias reports whether SQL aliases ("AS") should be truncated.
	KeepSQLAlias bool `mapstructure:"keep_sql_alias"`

	// DollarQuotedFunc reports whether to treat "$func$" delimited dollar-quoted strings
	// differently and not obfuscate them as a string. To read more about dollar quoted
	// strings see:
	//
	// https://www.postgresql.org/docs/current/sql-syntax-lexical.html#SQL-SYNTAX-DOLLAR-QUOTING
	DollarQuotedFunc bool `mapstructure:"dollar_quoted_func"`

	// ObfuscationMode specifies the obfuscation mode to use for go-sqllexer pkg.
	// When specified, obfuscator will attempt to use go-sqllexer pkg to obfuscate (and normalize) SQL queries.
	// Valid values are "normalize_only", "obfuscate_only", "obfuscate_and_normalize"
	ObfuscationMode obfuscate.ObfuscationMode `mapstructure:"obfuscation_mode"`

	// RemoveSpaceBetweenParentheses specifies whether to remove spaces between parentheses.
	// By default, spaces are inserted between parentheses during normalization.
	// This option is only valid when ObfuscationMode is "normalize_only" or "obfuscate_and_normalize".
	RemoveSpaceBetweenParentheses bool `mapstructure:"remove_space_between_parentheses"`

	// KeepNull specifies whether to disable obfuscate NULL value with ?.
	// This option is only valid when ObfuscationMode is "obfuscate_only" or "obfuscate_and_normalize".
	KeepNull bool `mapstructure:"keep_null"`

	// KeepBoolean specifies whether to disable obfuscate boolean value with ?.
	// This option is only valid when ObfuscationMode is "obfuscate_only" or "obfuscate_and_normalize".
	KeepBoolean bool `mapstructure:"keep_boolean"`

	// KeepPositionalParameter specifies whether to disable obfuscate positional parameter with ?.
	// This option is only valid when ObfuscationMode is "obfuscate_only" or "obfuscate_and_normalize".
	KeepPositionalParameter bool `mapstructure:"keep_positional_parameter"`

	// KeepTrailingSemicolon specifies whether to keep trailing semicolon.
	// By default, trailing semicolon is removed during normalization.
	// This option is only valid when ObfuscationMode is "normalize_only" or "obfuscate_and_normalize".
	KeepTrailingSemicolon bool `mapstructure:"keep_trailing_semicolon"`

	// KeepIdentifierQuotation specifies whether to keep identifier quotation, e.g. "my_table" or [my_table].
	// By default, identifier quotation is removed during normalization.
	// This option is only valid when ObfuscationMode is "normalize_only" or "obfuscate_and_normalize".
	KeepIdentifierQuotation bool `mapstructure:"keep_identifier_quotation"`
}

// Export returns an obfuscate.Config matching o.
func (o *ObfuscationConfig) Export() obfuscate.Config {
	if !o.Enabled {
		return obfuscate.Config{}
	}

	return obfuscate.Config{
		SQL:                  o.SQL.Export(),
		ES:                   o.ES,
		OpenSearch:           o.OpenSearch,
		Mongo:                o.Mongo,
		SQLExecPlan:          o.SQLExecPlan,
		SQLExecPlanNormalize: o.SQLExecPlanNormalize,
		HTTP:                 o.HTTP,
		Redis:                o.Redis,
		Memcached:            o.Memcached,
		CreditCard:           o.CreditCards,
	}
}

// Export returns an obfuscate.Config matching o.
func (o *SQLConfig) Export() obfuscate.SQLConfig {
	return obfuscate.SQLConfig{
		DBMS:                          o.DBMS,
		TableNames:                    o.TableNames,
		CollectCommands:               o.CollectCommands,
		CollectComments:               o.CollectComments,
		CollectProcedures:             o.CollectProcedures,
		ReplaceDigits:                 o.ReplaceDigits,
		KeepSQLAlias:                  o.KeepSQLAlias,
		DollarQuotedFunc:              o.DollarQuotedFunc,
		ObfuscationMode:               o.ObfuscationMode,
		KeepNull:                      o.KeepNull,
		KeepBoolean:                   o.KeepBoolean,
		KeepPositionalParameter:       o.KeepPositionalParameter,
		KeepTrailingSemicolon:         o.KeepTrailingSemicolon,
		KeepIdentifierQuotation:       o.KeepIdentifierQuotation,
		RemoveSpaceBetweenParentheses: o.RemoveSpaceBetweenParentheses,
	}
}
