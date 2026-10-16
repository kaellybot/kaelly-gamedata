package storages

import (
	"errors"
	"io/fs"
	"os"
	"path"

	"github.com/kaellybot/kaelly-gamedata/models/constants"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

func New() (*Impl, error) {
	storagePath := viper.GetString(constants.StoragePath)
	if err := os.MkdirAll(storagePath, dirPermission); err != nil {
		return nil, err
	}

	root, err := os.OpenRoot(storagePath)
	if err != nil {
		return nil, err
	}

	return &Impl{root: root}, nil
}

func (service *Impl) Exists(key string) bool {
	_, err := service.root.Stat(key)
	return err == nil
}

func (service *Impl) Put(key string, body []byte) error {
	if err := service.root.MkdirAll(path.Dir(key), dirPermission); err != nil {
		return err
	}

	return service.root.WriteFile(key, body, filePermission)
}

func (service *Impl) Get(key string) ([]byte, error) {
	return service.root.ReadFile(key)
}

func (service *Impl) Delete(key string) error {
	err := service.root.Remove(key)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}

	return err
}

// List returns the keys of the files under prefix. A missing prefix is empty.
func (service *Impl) List(prefix string) ([]string, error) {
	keys := make([]string, 0)
	err := fs.WalkDir(service.root.FS(), prefix, func(key string, entry fs.DirEntry, errWalk error) error {
		if errWalk != nil {
			return errWalk
		}

		if !entry.IsDir() {
			keys = append(keys, key)
		}
		return nil
	})

	if errors.Is(err, fs.ErrNotExist) {
		return keys, nil
	}

	return keys, err
}

func (service *Impl) Shutdown() {
	if err := service.root.Close(); err != nil {
		log.Error().Err(err).Msgf("Failed to close the storage")
	}
}
