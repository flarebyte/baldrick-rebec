package message

import (
	yaml "gopkg.in/yaml.v3"
)

func yamlUnmarshalImpl(b []byte, v any) error {
	return yaml.Unmarshal(b, v)
}
