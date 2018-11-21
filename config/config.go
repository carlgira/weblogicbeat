// Config is put into a different package to prevent cyclic imports in case
// it is needed in several locations

package config

import "time"

type Config struct {
	Period       time.Duration `config:"period"`
	Host         string        `config:"host"`
	Username     string        `config:"username"`
	Password     string        `config:"password"`
	ServerNames  []string      `config:"servernames"`
	Datasources  []string      `config:"datasources"`
	Applications []string      `config:"applications"`
}

var DefaultConfig = Config{
	Period:       1 * time.Second,
	Host:         "",
	Username:     "",
	Password:     "",
	ServerNames:  []string{},
	Datasources:  []string{},
	Applications: []string{},
}
