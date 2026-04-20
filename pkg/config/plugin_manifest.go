package config

type PluginManifest struct {
	Entry     string
	Codec     string
	Frontends []string
	Targets   []string
}
