package config

import (
	"bytes"

	"gopkg.in/yaml.v3"
)

func newDecoder(b []byte) *yaml.Decoder {
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	return dec
}
