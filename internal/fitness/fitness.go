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
	Skeleton           profile.Skeleton    `json:"skeleton"`
}

func Read(dbh *sql.DB) *Fitness {
	profileData := profile.Read()
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

	skeleton, err := profile.ReadSkeleton()
	if err != nil {
		// If skeleton doesn't exist or can't be read, use empty skeleton
		skeleton = &profile.Skeleton{}
	}

	fitness := Fitness{
		Profile:            profileData,
		ActivitiesThisWeek: thisWeek,
		ActivitiesLastWeek: lastWeek,
		ActivitiesOlder:    older,
		Skeleton:           *skeleton,
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
