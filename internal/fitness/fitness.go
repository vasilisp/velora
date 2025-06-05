package fitness

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/invopop/jsonschema"
	"github.com/vasilisp/velora/internal/db"
	"github.com/vasilisp/velora/internal/profile"
	"github.com/vasilisp/velora/internal/util"
)

type Fitness struct {
	profile.Profile
	ActivitiesThisWeek []db.ActivityUnsafe `json:"activities_this_week"`
	ActivitiesLastWeek []db.ActivityUnsafe `json:"activities_last_week"`
	ActivitiesOlder    []db.ActivityUnsafe `json:"activities_older"`
}

func Read(dbh *sql.DB) *Fitness {
	profile := profile.Read()
	startOfWeek := util.BeginningOfWeek(time.Now())
	startOfLastWeek := startOfWeek.AddDate(0, 0, -7)

	activities, err := db.LastActivities(dbh, 60)
	if err != nil {
		util.Fatalf("error getting activities: %v\n", err)
	}

	var thisWeek, lastWeek, older []db.ActivityUnsafe
	for _, activity := range activities {
		switch {
		case activity.Time.After(startOfWeek):
			thisWeek = append(thisWeek, activity)
		case activity.Time.After(startOfLastWeek):
			lastWeek = append(lastWeek, activity)
		default:
			older = append(older, activity)
		}
	}

	fitness := Fitness{
		Profile:            profile,
		ActivitiesThisWeek: thisWeek,
		ActivitiesLastWeek: lastWeek,
		ActivitiesOlder:    older,
	}

	return &fitness
}

// JSONSchema returns the JSON schema for the Fitness struct
func JSONSchema() string {
	reflector := jsonschema.Reflector{}
	schema := reflector.Reflect(&Fitness{})

	schemaBytes, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		util.Fatalf("error marshalling schema: %v\n", err)
	}

	return string(schemaBytes)
}
