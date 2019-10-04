package config

import (
	"os"

	"github.com/jinzhu/configor"
	"github.com/pkg/errors"
)

type Config struct {
	Graylog Graylog `yaml:"graylog"`
	Service Service `yaml:"service"`
}

type Graylog struct {
	Host     string `yaml:"host" required:"true"`
	Port     int    `yaml:"port" required:"true"`
	Platform string `yaml:"platform"`

	EnableFileLog bool   `yaml:"enableFileLog"`
	NameLogFile   string `yaml:"nameLogFile"`
	DisableColor  bool   `yaml:"disableColor"`
	DebugLvl      bool   `yaml:"debugLvl"`
}

type Service struct {
	Host          string `yaml:"host" required:"true"`
	Port          string `yaml:"port" required:"true"`
	MaxCntConnect int    `yaml:"maxCntConnect" required:"true"`

	MaxLenBuffer int `yaml:"maxLenBuffer" required:"true"`
	ReadTimeout  int `yaml:"readTimeout" required:"true"`
	WriteTimeout int `yaml:"writeTimeout" required:"true"`

	CacheTTL   int `yaml:"cacheTTL" required:"true"`
	CacheReset int `yaml:"cacheReset" required:"true"`

	ErrorMsgTemplate string              `yaml:"errorMsgTemplate" required:"true"`
	DefaultWhois     string              `yaml:"defaultWhois" required:"true"`
	DomainZoneWhois  map[string]string   `yaml:"domainZoneWhois" required:"true"`
	AddWhoisDescInfo map[string][]string `yaml:"addWhoisDescInfo" required:"true"`
}

func Load(filename string) (Config, error) {
	if _, err := os.Stat(filename); err != nil {
		return Config{}, errors.WithMessage(err, "failed to stat config file")
	}

	var config Config
	if err := configor.Load(&config, filename); err != nil {
		return Config{}, errors.WithMessage(err, "failed to load config")
	}

	return config, nil
}
