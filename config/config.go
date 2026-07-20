package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Collection struct {
	Path             string            `yaml:"path"`
	Pattern          string            `yaml:"pattern"`
	Ignore           []string          `yaml:"ignore,omitempty"`
	Context          map[string]string `yaml:"context,omitempty"`
	Update           string            `yaml:"update,omitempty"`
	IncludeByDefault *bool             `yaml:"includeByDefault,omitempty"`
}

type ModelsConfig struct {
	Embed    string `yaml:"embed,omitempty"`
	Rerank   string `yaml:"rerank,omitempty"`
	Generate string `yaml:"generate,omitempty"`
}

type CollectionConfig struct {
	GlobalContext     string                `yaml:"global_context,omitempty"`
	EditorURI         string                `yaml:"editor_uri,omitempty"`
	EditorURITemplate string                `yaml:"editor_uri_template,omitempty"`
	Collections       map[string]Collection `yaml:"collections"`
	Models            *ModelsConfig         `yaml:"models,omitempty"`
}

type NamedCollection struct {
	Name string
	Collection
}

var (
	currentIndexName = "index"
	configSource     struct {
		SourceType string // "file" or "inline"
		FilePath   string
		InlineConf *CollectionConfig
	}
)

func init() {
	configSource.SourceType = "file"
}

func QmdHomedir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	if h := os.Getenv("USERPROFILE"); h != "" {
		return h
	}
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return "/tmp"
}

func SetConfigIndexName(name string) {
	if strings.Contains(name, "/") {
		// Resolve relative path to absolute
		abs, err := filepath.Abs(name)
		if err == nil {
			name = abs
		}
		// Replace path separators with underscores to create a valid filename
		currentIndexName = strings.TrimPrefix(strings.ReplaceAll(name, "/", "_"), "_")
	} else {
		currentIndexName = name
	}
}

func GetConfigIndexName() string {
	return currentIndexName
}

func SetConfigSourceFile(path string) {
	configSource.SourceType = "file"
	configSource.FilePath = path
	configSource.InlineConf = nil
}

func SetConfigSourceInline(conf *CollectionConfig) {
	configSource.SourceType = "inline"
	configSource.FilePath = ""
	configSource.InlineConf = conf
}

func ResetConfigSource() {
	configSource.SourceType = "file"
	configSource.FilePath = ""
	configSource.InlineConf = nil
}

func GetConfigDir() string {
	if dir := os.Getenv("QMD_CONFIG_DIR"); dir != "" {
		return dir
	}
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "qmd")
	}
	return filepath.Join(QmdHomedir(), ".config", "qmd")
}

func GetConfigFilePath() string {
	if configSource.SourceType == "file" && configSource.FilePath != "" {
		return configSource.FilePath
	}
	return filepath.Join(GetConfigDir(), currentIndexName+".yml")
}

func FindLocalConfigPath(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}

	for {
		qmdDir := filepath.Join(dir, ".qmd")
		yamlPath := filepath.Join(qmdDir, "index.yaml")
		if _, err := os.Stat(yamlPath); err == nil {
			return yamlPath, nil
		}
		ymlPath := filepath.Join(qmdDir, "index.yml")
		if _, err := os.Stat(ymlPath); err == nil {
			return ymlPath, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", errors.New("no local config path found")
}

func GetLocalDbPath(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), "index.sqlite")
}

func LoadConfig() (*CollectionConfig, error) {
	if configSource.SourceType == "inline" {
		if configSource.InlineConf == nil {
			return &CollectionConfig{Collections: make(map[string]Collection)}, nil
		}
		if configSource.InlineConf.Collections == nil {
			configSource.InlineConf.Collections = make(map[string]Collection)
		}
		return configSource.InlineConf, nil
	}

	configPath := GetConfigFilePath()
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return &CollectionConfig{Collections: make(map[string]Collection)}, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Treat empty file as empty config
	if len(strings.TrimSpace(string(data))) == 0 {
		return &CollectionConfig{Collections: make(map[string]Collection)}, nil
	}

	var config CollectionConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config %s: %w", configPath, err)
	}

	if config.Collections == nil {
		config.Collections = make(map[string]Collection)
	}

	return &config, nil
}

func SaveConfig(config *CollectionConfig) error {
	if configSource.SourceType == "inline" {
		configSource.InlineConf = config
		return nil
	}

	configPath := GetConfigFilePath()
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to serialize config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config %s: %w", configPath, err)
	}

	return nil
}

func GetCollection(name string) (*NamedCollection, error) {
	config, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	coll, exists := config.Collections[name]
	if !exists {
		return nil, nil
	}

	return &NamedCollection{Name: name, Collection: coll}, nil
}

func ListCollections() ([]NamedCollection, error) {
	config, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	var list []NamedCollection
	for name, coll := range config.Collections {
		list = append(list, NamedCollection{Name: name, Collection: coll})
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].Name < list[j].Name
	})

	return list, nil
}

func GetDefaultCollections() ([]NamedCollection, error) {
	colls, err := ListCollections()
	if err != nil {
		return nil, err
	}

	var filtered []NamedCollection
	for _, coll := range colls {
		if coll.IncludeByDefault == nil || *coll.IncludeByDefault {
			filtered = append(filtered, coll)
		}
	}
	return filtered, nil
}

func GetDefaultCollectionNames() ([]string, error) {
	colls, err := GetDefaultCollections()
	if err != nil {
		return nil, err
	}

	var names []string
	for _, coll := range colls {
		names = append(names, coll.Name)
	}
	return names, nil
}

type CollectionSettings struct {
	Update           *string
	IncludeByDefault *bool
}

func UpdateCollectionSettings(name string, settings CollectionSettings) (bool, error) {
	config, err := LoadConfig()
	if err != nil {
		return false, err
	}

	collection, exists := config.Collections[name]
	if !exists {
		return false, nil
	}

	if settings.Update != nil {
		if *settings.Update == "" {
			collection.Update = ""
		} else {
			collection.Update = *settings.Update
		}
	}

	if settings.IncludeByDefault != nil {
		if *settings.IncludeByDefault {
			collection.IncludeByDefault = nil
		} else {
			val := false
			collection.IncludeByDefault = &val
		}
	}

	config.Collections[name] = collection
	if err := SaveConfig(config); err != nil {
		return false, err
	}

	return true, nil
}

func AddCollection(name, path, pattern string) error {
	config, err := LoadConfig()
	if err != nil {
		return err
	}

	coll := config.Collections[name]
	coll.Path = path
	coll.Pattern = pattern

	config.Collections[name] = coll
	return SaveConfig(config)
}

func RemoveCollection(name string) (bool, error) {
	config, err := LoadConfig()
	if err != nil {
		return false, err
	}

	if _, exists := config.Collections[name]; !exists {
		return false, nil
	}

	delete(config.Collections, name)
	if err := SaveConfig(config); err != nil {
		return false, err
	}

	return true, nil
}

func RenameCollection(oldName, newName string) (bool, error) {
	config, err := LoadConfig()
	if err != nil {
		return false, err
	}

	coll, exists := config.Collections[oldName]
	if !exists {
		return false, nil
	}

	if _, existsNew := config.Collections[newName]; existsNew {
		return false, fmt.Errorf("Collection '%s' already exists", newName)
	}

	config.Collections[newName] = coll
	delete(config.Collections, oldName)

	if err := SaveConfig(config); err != nil {
		return false, err
	}

	return true, nil
}

func GetGlobalContext() (string, error) {
	config, err := LoadConfig()
	if err != nil {
		return "", err
	}
	return config.GlobalContext, nil
}

func SetGlobalContext(context string) error {
	config, err := LoadConfig()
	if err != nil {
		return err
	}
	config.GlobalContext = context
	return SaveConfig(config)
}

func AddContext(collectionName, pathPrefix, contextText string) (bool, error) {
	config, err := LoadConfig()
	if err != nil {
		return false, err
	}

	coll, exists := config.Collections[collectionName]
	if !exists {
		return false, nil
	}

	if coll.Context == nil {
		coll.Context = make(map[string]string)
	}

	coll.Context[pathPrefix] = contextText
	config.Collections[collectionName] = coll

	if err := SaveConfig(config); err != nil {
		return false, err
	}

	return true, nil
}

func RemoveContext(collectionName, pathPrefix string) (bool, error) {
	config, err := LoadConfig()
	if err != nil {
		return false, err
	}

	coll, exists := config.Collections[collectionName]
	if !exists || coll.Context == nil {
		return false, nil
	}

	if _, ok := coll.Context[pathPrefix]; !ok {
		return false, nil
	}

	delete(coll.Context, pathPrefix)
	if len(coll.Context) == 0 {
		coll.Context = nil
	}

	config.Collections[collectionName] = coll
	if err := SaveConfig(config); err != nil {
		return false, err
	}

	return true, nil
}

type ContextRecord struct {
	Collection string
	Path       string
	Context    string
}

func ListAllContexts() ([]ContextRecord, error) {
	config, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	var results []ContextRecord
	if config.GlobalContext != "" {
		results = append(results, ContextRecord{
			Collection: "*",
			Path:       "/",
			Context:    config.GlobalContext,
		})
	}

	for name, coll := range config.Collections {
		if coll.Context != nil {
			for path, ctx := range coll.Context {
				results = append(results, ContextRecord{
					Collection: name,
					Path:       path,
					Context:    ctx,
				})
			}
		}
	}

	// Sort results to be deterministic: first by Collection, then by Path
	sort.Slice(results, func(i, j int) bool {
		if results[i].Collection != results[j].Collection {
			return results[i].Collection < results[j].Collection
		}
		return results[i].Path < results[j].Path
	})

	return results, nil
}

func FindContextForPath(collectionName, filePath string) (string, error) {
	config, err := LoadConfig()
	if err != nil {
		return "", err
	}

	coll, exists := config.Collections[collectionName]
	if !exists || coll.Context == nil {
		return config.GlobalContext, nil
	}

	var matches []struct {
		prefix  string
		context string
	}

	for prefix, context := range coll.Context {
		normalizedPath := filePath
		if !strings.HasPrefix(normalizedPath, "/") {
			normalizedPath = "/" + normalizedPath
		}
		normalizedPrefix := prefix
		if !strings.HasPrefix(normalizedPrefix, "/") {
			normalizedPrefix = "/" + normalizedPrefix
		}

		if strings.HasPrefix(normalizedPath, normalizedPrefix) {
			matches = append(matches, struct {
				prefix  string
				context string
			}{prefix: normalizedPrefix, context: context})
		}
	}

	if len(matches) > 0 {
		sort.Slice(matches, func(i, j int) bool {
			return len(matches[i].prefix) > len(matches[j].prefix)
		})
		return matches[0].context, nil
	}

	return config.GlobalContext, nil
}

func ConfigExists() bool {
	if configSource.SourceType == "inline" {
		return true
	}
	path := GetConfigFilePath()
	_, err := os.Stat(path)
	return err == nil
}

func IsValidCollectionName(name string) bool {
	reg := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	return reg.MatchString(name)
}
