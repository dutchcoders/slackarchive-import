package config

import (
	"io/ioutil"

	"gopkg.in/yaml.v2"

	"github.com/codegangsta/cli"
	"github.com/imdario/mergo"
)

var config Config = DefaultConfig

func Load(c *cli.Context) error {
	var err error
	if err = config.Load(c.GlobalString("config")); err != nil {
		return err
	}

	// initialize
	return mergo.MergeWithOverwrite(&config, &Config{
		DSN: c.GlobalString("dsn"),
	})
}

func Get() *Config {
	return &config
}

type Config struct {
	DSN             string `yaml:"dsn"`
	DiscoveryURL    string `yaml:"discovery-url"`
	ListenPeerURL   string `yaml:"listenpeer-url"`
	ListenClientURL string `yaml:"listenclient-url"`

	DataDir string `yaml:"data-dir"`
	Name    string `yaml:"name"`

	ElasticSearch struct {
		Host string `yaml:"host"`
	} `yaml:"elasticsearch"`
	Tokens []string `yaml:"tokens"`
}

var DefaultConfig Config = Config{}

func (c *Config) Load(path string) error {
	var data []byte
	var err error
	if data, err = ioutil.ReadFile(path); err != nil {
		return err
	}

	tmp := Config{}
	if err = yaml.Unmarshal([]byte(data), &tmp); err != nil {
		return err
	}

	return mergo.MergeWithOverwrite(c, tmp)
}
