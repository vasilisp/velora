package fitness

import (
	"database/sql"

	"github.com/vasilisp/velora/internal/db"
	"github.com/vasilisp/velora/internal/profile"
	"github.com/vasilisp/velora/internal/util"
)

type Fitness struct {
	profile.Profile
	Activities []db.ActivityUnsafe `json:"activities"`
}

func Read(dbh *sql.DB) Fitness {
	profile := profile.Read()

	activities, err := db.LastActivities(dbh, 10)
	if err != nil {
		util.Fatalf("error getting last activities: %v\n", err)
	}

	return Fitness{
		Profile:    profile,
		Activities: activities,
	}
}
