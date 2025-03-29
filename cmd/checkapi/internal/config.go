package internal

type Function struct {
	Name        string   `json:"name"`
	Receiver    string   `json:"receiver"`
	ReturnTypes []string `json:"return_types,omitempty"`
	ParamTypes  []string `json:"param_types,omitempty"`
}

type Apistruct struct {
	Name   string   `json:"name"`
	Fields []string `json:"fields"`
}

type Api struct {
	Values    []string     `json:"values,omitempty"`
	Structs   []*Apistruct `json:"structs,omitempty"`
	Functions []*Function  `json:"functions,omitempty"`
}

type FunctionDescription struct {
	Classes     []string `yaml:"classes"`
	Name        string   `yaml:"name"`
	Parameters  []string `yaml:"parameters"`
	ReturnTypes []string `yaml:"return_types"`
}

type Config struct {
	IgnoredPaths     []string              `yaml:"ignored_paths"`
	AllowedFunctions []FunctionDescription `yaml:"allowed_functions"`
	IgnoredFunctions []string              `yaml:"ignored_functions"`
	UnkeyedLiteral   UnkeyedLiteral        `yaml:"unkeyed_literal_initialization"`
}

type UnkeyedLiteral struct {
	Enabled bool `yaml:"enabled"`
	Limit   int  `yaml:"limit"`
}
