package appconf

import (
	"errors"
	"github.com/google/uuid"
	"github.com/xetys/hetzner-kube/types"
	"gopkg.in/yaml.v3"
	"os"
	"path"
)

type vault struct {
	Configs map[uuid.UUID]types.ClusterConfig `yaml:"configs"`
}

func GetConfig(name string) (types.ClusterConfig, error) {
	v, err := loadVault()
	if err != nil {
		return types.ClusterConfig{}, err
	}

	var config types.ClusterConfig
	for _, clusterConfig := range v.Configs {
		if clusterConfig.ClusterName != name {
			continue
		}

		config = clusterConfig
	}

	return config, nil
}

// const confPath = "~/.hetzner-kube/"
const confPath = "./"
const confName = "config.yaml"

func SaveConfig(config types.ClusterConfig) error {
	v, err := loadVault()
	v.Configs[config.GetUUID()] = config

	d, err := yaml.Marshal(&v)
	if err != nil {
		return err
	}

	//TODO Different OS support
	err = os.WriteFile(path.Join(confPath, confName), d, 0644)
	if err != nil {
		return err
	}

	return nil
}

func loadVault() (vault, error) {
	_, err := os.Stat(confPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			err := os.Mkdir(confPath, 0644)
			if err != nil {
				return vault{}, err
			}
		} else {
			return vault{}, err
		}
	}

	_, err = os.Stat(path.Join(confPath, confName))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			_, err := os.Create(confPath)
			if err != nil {
				return vault{}, err
			}
		} else {
			return vault{}, err
		}
	}

	file, err := os.ReadFile(path.Join(confPath, confName))
	if err != nil {
		return vault{}, err
	}

	var conf vault
	err = yaml.Unmarshal(file, &conf)
	return conf, err
}
