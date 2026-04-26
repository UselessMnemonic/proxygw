package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/alecthomas/kingpin/v2"
	"gopkg.in/yaml.v3"

	"proxygw/pkg/config"
	"proxygw/plugin/ctl/ipc"
	"proxygw/plugin/ctl/ipc/codec"
)

func main() {
	app := kingpin.New("proxygwctl", "Proxy gateway control client")

	configPath := app.Flag("config", "path to runtime configuration").
		Default("/etc/proxygw.yaml").
		String()

	statusCmd := app.Command("status", "Get proxy gateway status")

	command, err := app.Parse(os.Args[1:])
	if err != nil {
		log.Printf("proxygwctl: %v", err)
		os.Exit(1)
	}

	client, err := dialClient(*configPath)
	if err != nil {
		log.Printf("proxygwctl: %v", err)
		os.Exit(1)
	}
	defer client.Close()

	switch command {
	case statusCmd.FullCommand():
		err = runStatus(client)
	default:
		err = fmt.Errorf("unsupported command %q", command)
	}
	if err != nil {
		log.Printf("proxygwctl: %v", err)
		os.Exit(1)
	}
}

func runStatus(client *Client) error {
	resp, err := client.StatusRequest()
	if err != nil {
		return fmt.Errorf("status request: %w", err)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(resp); err != nil {
		return fmt.Errorf("write status output: %w", err)
	}
	return nil
}

func dialClient(configPath string) (*Client, error) {
	cfg, err := loadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config %q: %w", configPath, err)
	}

	ctlConfig, ok := cfg.Plugins["ctl"]
	if !ok {
		return nil, fmt.Errorf("config %q does not define plugins.ctl", configPath)
	}

	socket, ok := ctlConfig["socket"].(string)
	if !ok || socket == "" {
		return nil, fmt.Errorf("config %q is missing plugins.ctl.socket", configPath)
	}

	c := "json"
	if rawCodec, ok := ctlConfig["codec"].(string); ok && rawCodec != "" {
		c = rawCodec
	}

	codec := codec.FindByName(c)
	if codec == nil {
		return nil, fmt.Errorf("config %q has invalid plugins.ctl.codec %q", configPath, c)
	}

	conn, err := ipc.Dial("unix", socket, codec)
	if err != nil {
		return nil, fmt.Errorf("dial ctl socket %q: %w", socket, err)
	}
	return NewClient(conn), nil
}

func loadConfig(path string) (*config.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &config.Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}
