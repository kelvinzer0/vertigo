package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the structure of the configuration file.
type Config struct {
	Server struct {
		Port int    `yaml:"port"`
		Host string `yaml:"host"`
	} `yaml:"server"`
	Gemini struct {
		APIKeys []string `yaml:"api_keys"`
	} `yaml:"gemini"`
}

// Load reads a YAML file from the given path and unmarshals it into a Config struct.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
