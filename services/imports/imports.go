package imports

import (
	amqp "github.com/kaellybot/kaelly-amqp"
	"github.com/kaellybot/kaelly-gamedata/models/constants"
	"github.com/rs/zerolog/log"
)

func New() (*Impl, error) {
	return &Impl{}, nil
}

// Import runs one pass of the import described in docs/specs/dofus-touch-data.md (§5).
// Nothing is downloaded yet: phase 1 (almanax) comes next.
func (service *Impl) Import() error {
	log.Info().
		Str(constants.LogGame, amqp.Game_DOFUS_TOUCH.String()).
		Msgf("Importing game data...")

	return nil
}
