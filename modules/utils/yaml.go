package utils

import (
	"os"
	"gopkg.in/yaml.v3"
)

// Reads YAML-File into specified struct
func ReadYAMLIntoStruct[T any](filepath string) (T, error) {
	var element T

	var data []byte
	data, err := os.ReadFile(filepath)
	if err != nil {
		return element, err
	}

	if err := yaml.Unmarshal(data, &element); err != nil {
		return element, err
	}

	return element, nil
}