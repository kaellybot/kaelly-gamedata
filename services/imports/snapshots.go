package imports

import (
	"bytes"
	"compress/gzip"
	"fmt"

	"github.com/kaellybot/kaelly-gamedata/models/constants"
	"github.com/kaellybot/kaelly-gamedata/models/entities"
	"github.com/kaellybot/kaelly-gamedata/models/mappers"
	"github.com/rs/zerolog/log"
)

// storeSnapshots writes the raw files, content-addressed so an unchanged file is
// stored once, and lists them on the import. Nothing is written in dry run.
func (service *Impl) storeSnapshots(record *entities.GameDataImport, downloads []download) error {
	for _, file := range downloads {
		hash := mappers.FileHash(file.body)
		key := fmt.Sprintf(snapshotKeyFormat, hash)
		if !service.dryRun && !service.storageService.Exists(key) {
			compressed, err := compress(file.body)
			if err != nil {
				return err
			}

			if errPut := service.storageService.Put(key, compressed); errPut != nil {
				return errPut
			}
		}

		record.Files = append(record.Files, entities.GameDataImportFile{
			Class:     file.class,
			Locale:    constants.GetLanguages()[file.language],
			Sha256:    hash,
			SizeBytes: len(file.body),
			ObjectKey: key,
		})
	}

	return nil
}

// cleanSnapshots deletes the raw files no longer referenced by the last applied imports.
// A failure is only logged: the snapshots are a convenience, not served data.
func (service *Impl) cleanSnapshots() {
	if service.dryRun {
		return
	}

	kept := make(map[string]struct{})
	applied := []entities.ImportStatus{entities.ImportStatusApplied, entities.ImportStatusPartial}
	for _, source := range []entities.ImportSource{entities.ImportSourceClient, entities.ImportSourceKrosmoz} {
		keys, err := service.importRepo.GetObjectKeys(constants.Game, source, applied, service.snapshotRetention)
		if err != nil {
			log.Error().Err(err).Msgf("Cannot read the snapshots to keep, cleaning skipped")
			return
		}

		for _, key := range keys {
			kept[key] = struct{}{}
		}
	}

	stored, err := service.storageService.List(snapshotPrefix)
	if err != nil {
		log.Error().Err(err).Msgf("Cannot list the snapshots, cleaning skipped")
		return
	}

	for _, key := range stored {
		if _, found := kept[key]; found {
			continue
		}

		if errDelete := service.storageService.Delete(key); errDelete != nil {
			log.Error().Err(errDelete).Str(constants.LogFileName, key).Msgf("Cannot delete a snapshot, continuing")
		}
	}
}

func compress(body []byte) ([]byte, error) {
	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	if _, err := writer.Write(body); err != nil {
		return nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}
