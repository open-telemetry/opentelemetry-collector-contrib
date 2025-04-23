package main

import (
	"bytes"
	"encoding"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"golang.org/x/mod/modfile"
	"sigs.k8s.io/yaml"
)

type Args struct {
	BasePrefix     string
	Dir            string
	SkippedModules CommaSeparatedSet
}

// CommaSeparatedSet is a custom type for parsing comma-separated values.
// Duplicate entries are ignored.
type CommaSeparatedSet map[string]struct{}

var _ encoding.TextMarshaler = (*CommaSeparatedSet)(nil)

func (c CommaSeparatedSet) MarshalText() ([]byte, error) {
	keys := make([]string, 0, len(c))
	for key := range c {
		keys = append(keys, key)
	}
	return []byte(strings.Join(keys, ",")), nil
}

var _ encoding.TextUnmarshaler = (*CommaSeparatedSet)(nil)

func (c *CommaSeparatedSet) UnmarshalText(text []byte) error {
	*c = make(map[string]struct{})
	for _, key := range strings.Split(string(text), ",") {
		key = strings.TrimSpace(key)
		if key != "" {
			(*c)[key] = struct{}{}
		} else {
			return fmt.Errorf("empty key in comma-separated list")
		}
	}
	return nil
}

func setupCLI() (Args, error) {
	cli := Args{}
	flag.StringVar(&cli.BasePrefix, "base-prefix", "", "The base prefix of your Go modules (e.g., github.com/yourorg)")
	flag.StringVar(&cli.Dir, "dir", ".", "The directory to scan for go.mod files")
	flag.TextVar(&cli.SkippedModules, "skipped-modules", CommaSeparatedSet{}, "Comma-separated list of Go modules to skip using glob expressions (e.g., rel/path/from/base-prefix/*/example)")
	flag.Parse()

	if cli.BasePrefix == "" {
		return cli, fmt.Errorf("basePrefix is required")
	}
	if cli.Dir == "" {
		return cli, fmt.Errorf("dir is required")
	}

	return cli, nil
}

func main() {
	args, err := setupCLI()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Generate the .codecov.yaml
	config, err := walkTree(args)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Add component list to the config
	err = addComponentList(config)
	if err != nil {
		fmt.Println("Error adding component list:", err)
	}
}

// Component represents a component in the Codecov configuration.
// Use JSON tags instead of YAML tags since this is what sigs.k8s.io/yaml supports.
type Component struct {
	ComponentID string   `json:"component_id"`
	Name        string   `json:"name"`
	Paths       []string `json:"paths"`
}

type ComponentManagement struct {
	IndividualComponents []Component `json:"individual_components"`
}

type CodecovConfig struct {
	ComponentManagement ComponentManagement `json:"component_management"`
}

var (
	// validFlagRegexp is a regular expression to validate codecov flags.
	// It is mentioned here: https://docs.codecov.com/docs/flags
	// It's unclear if components fit this pattern, but they seem to work with it.
	validFlagRegexp = regexp.MustCompile(`^[\w\.\-]{1,45}$`)

	// defaultSuffixList is a list of suffixes to remove from the module name.
	// We remove the suffixes so that the component ID is shorter.
	defaultSuffixList = []string{
		"receiver",
		"exporter",
		"encodingextension", // needs to be before "extension"
		"extension",
		"processor",
		"connector",
		"provider",
		"scraper",
	}
)

func generateComponentID(moduleName string, cli Args) (string, error) {
	componentID := strings.Replace(moduleName, cli.BasePrefix+"/", "", 1)
	for _, suffix := range defaultSuffixList {
		componentID = strings.TrimSuffix(componentID, suffix)
	}
	componentID = strings.Replace(componentID, "/", "_", -1)

	if !validFlagRegexp.MatchString(componentID) {
		return "", fmt.Errorf("component ID %q does not match the required pattern", componentID)
	}
	return componentID, nil
}

// walkTree uses filepath.Walk to recursively traverse the base directory looking for go.mod files
func walkTree(cli Args) (*CodecovConfig, error) {
	config := &CodecovConfig{}

	err := filepath.Walk(cli.Dir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && path != cli.Dir {
			goModPath := filepath.Join(path, "go.mod")
			if _, err := os.Stat(goModPath); os.IsNotExist(err) {
				return nil
			}

			moduleName, err := readModuleName(goModPath)
			if err != nil {
				return err
			}

			// Skip if the module is in the skipped list
			relModName := strings.Replace(moduleName, cli.BasePrefix+"/", "", 1)
			for pattern := range cli.SkippedModules {
				matched, err := doublestar.Match(pattern, relModName)
				if err != nil {
					return fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
				}
				if matched {
					fmt.Printf("Skipping module %q\n", relModName)
					return nil
				}
			}

			// Create component and add it to the config
			relativePath, err := filepath.Rel(cli.Dir, path)
			if err != nil {
				return err
			}

			componentID, err := generateComponentID(moduleName, cli)
			if err != nil {
				return err
			}
			component := Component{
				ComponentID: componentID,
				Name:        componentID,
				Paths:       []string{relativePath + "/**"},
			}

			config.ComponentManagement.IndividualComponents = append(config.ComponentManagement.IndividualComponents, component)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return config, nil
}

// readModuleName reads the go.mod file and returns the module's name
func readModuleName(goModPath string) (string, error) {
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return "", fmt.Errorf("failed to read go.mod file %q: %w", goModPath, err)
	}

	modFile, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return "", fmt.Errorf("failed to parse go.mod file %q: %w", goModPath, err)
	}

	if modFile.Module == nil || modFile.Module.Mod.Path == "" {
		return "", fmt.Errorf("no module name found in go.mod %q", goModPath)
	}

	return modFile.Module.Mod.Path, nil
}

const codecovFileName = ".codecov.yml"
const startComponentList = `# Start components list`
const endComponentList = `# End components list`

func addComponentList(config *CodecovConfig) error {
	yamlData, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %w", err)
	}

	replacement := []byte(startComponentList + "\n" + string(yamlData) + endComponentList)
	codecovCfg, err := os.ReadFile(codecovFileName)
	if err != nil {
		return fmt.Errorf("failed to read %q: %w", codecovFileName, err)
	}
	matchOldContent := regexp.MustCompile("(?s)" + startComponentList + ".*" + endComponentList)
	oldContent := matchOldContent.FindSubmatch(codecovCfg)
	if len(oldContent) == 0 {
		return fmt.Errorf("failed to find start and end markers in .codecov.yaml")
	}

	codecovCfg = bytes.ReplaceAll(codecovCfg, oldContent[0], replacement)
	err = os.WriteFile(codecovFileName, codecovCfg, 0o600)
	if err != nil {
		return fmt.Errorf("failed to write to %q: %w", codecovFileName, err)
	}
	return nil
}
