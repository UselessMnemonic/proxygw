package config

type PluginManifest struct {
	Cmd       string
	Args      []string
	Codec     string
	Frontends []string
	Targets   []string
}
