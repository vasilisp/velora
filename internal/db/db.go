package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/vasilisp/lingograph/extra"
	"github.com/vasilisp/velora/internal/util"
)

type Sport uint

const (
	Running Sport = iota
	Cycling
	Swimming
)

func (s Sport) String() string {
	util.Assert(s >= Running && s <= Swimming, "invalid sport")
	return []string{"running", "cycling", "swimming"}[s]
}

func SportFromString(s string) (Sport, error) {
	switch strings.ToLower(s) {
	case "running":
		return Running, nil
	case "cycling":
		return Cycling, nil
	case "swimming":
		return Swimming, nil
	default:
		return Running, fmt.Errorf("invalid sport: %s", s)
	}
}

type Segment struct {
	Repeat   int `json:"repeat" jsonschema:"description=The number of times to repeat the segment. Can be 1."`
	Distance int `json:"distance" jsonschema:"description=The planned distance in meters"`
	Zone     int `json:"zone" jsonschema:"description=The planned zone (1-5)"`
}

type ActivityUnsafe struct {
	Time           time.Time `json:"time"`
	Duration       int       `json:"duration"`
	DurationTotal  int       `json:"duration_total,omitempty"`
	Distance       int       `json:"distance"`
	Sport          Sport     `json:"-"`
	VerticalGain   int       `json:"vertical_gain"`
	Notes          string    `json:"notes"`
	WasRecommended bool      `json:"was_recommended"`
	Segments       []Segment `json:"segments"`
}

func (a ActivityUnsafe) MarshalJSON() ([]byte, error) {
	type Alias ActivityUnsafe
	return json.Marshal(&struct {
		Sport string `json:"sport"`
		*Alias
	}{
		Sport: a.Sport.String(),
		Alias: (*Alias)(&a),
	})
}

func (a *ActivityUnsafe) UnmarshalJSON(data []byte) error {
	type Alias ActivityUnsafe
	aux := &struct {
		Sport string `json:"sport"`
		*Alias
	}{
		Alias: (*Alias)(a),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	sport, err := SportFromString(aux.Sport)
	if err != nil {
		return err
	}
	a.Sport = sport
	return nil
}

func formatSegments(segments []Segment) string {
	formatted := ""
	for _, segment := range segments {
		formatted += fmt.Sprintf("  - Repeat: %d, Distance: %d, Zone: %d\n", segment.Repeat, segment.Distance, segment.Zone)
	}
	return formatted
}

func (a *ActivityUnsafe) Show() string {
	util.Assert(a != nil, "Show nil activity")

	return fmt.Sprintf("Date: %s\nSport: %s\nTime: %s\nDistance: %s\nVertical Gain: %dm\nNotes: %s\nSegments: %v",
		a.Time.Format("Jan 2, 15:04"),
		a.Sport,
		util.FormatDuration(a.Duration),
		util.FormatDistance(a.Distance),
		a.VerticalGain,
		extra.SanitizeOutputString(a.Notes, true),
		formatSegments(a.Segments))
}

type activity struct {
	a ActivityUnsafe
}

func (a ActivityUnsafe) ToActivity() (activity, error) {
	if a.DurationTotal <= 0 {
		a.DurationTotal = a.Duration
	}

	activity := activity{a}
	var err error = nil

	if a.Duration <= 0 {
		err = fmt.Errorf("duration must be positive")
	}

	if a.Distance <= 0 {
		err = fmt.Errorf("distance must be positive")
	}

	if a.Sport < Running || a.Sport > Swimming {
		err = fmt.Errorf("invalid sport")
	}

	if a.VerticalGain < 0 {
		err = fmt.Errorf("verticalGain must be non-negative")
	}

	return activity, err
}

func LastActivities(db *sql.DB, limit int) ([]ActivityUnsafe, error) {
	util.Assert(limit > 0, "LastActivities non-positive limit")
	util.Assert(db != nil, "LastActivities nil db")

	rows, err := db.Query(`
		SELECT timestamp, duration, duration_total, sport, distance, vertical_gain, notes, was_recommended, segments
		FROM activities
		ORDER BY timestamp DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("error querying activities: %v", err)
	}
	defer rows.Close()

	activities := []ActivityUnsafe{}
	for rows.Next() {
		var sportStr string
		var activity ActivityUnsafe
		var verticalGain sql.NullInt64
		var segmentsBytes []byte

		err := rows.Scan(&activity.Time, &activity.Duration, &activity.DurationTotal, &sportStr, &activity.Distance, &verticalGain, &activity.Notes, &activity.WasRecommended, &segmentsBytes)
		if err != nil {
			return nil, fmt.Errorf("error scanning activity: %v", err)
		}

		sport, err := SportFromString(sportStr)
		if err != nil {
			return nil, fmt.Errorf("error parsing sport: %v", err)
		}
		activity.Sport = sport

		if verticalGain.Valid {
			activity.VerticalGain = int(verticalGain.Int64)
		}

		if len(segmentsBytes) > 0 {
			var segments []Segment
			err = json.Unmarshal(segmentsBytes, &segments)
			if err != nil {
				return nil, fmt.Errorf("error unmarshalling segments: %v", err)
			}
			activity.Segments = segments
		}

		activities = append(activities, activity)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating activities: %v", err)
	}

	return activities, nil
}

func Init() (*sql.DB, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		util.Fatalf("cannot locate home directory: %v", err)
	}

	dotDir := filepath.Join(homeDir, ".velora")
	if os.MkdirAll(dotDir, 0755) != nil {
		util.Fatalf("cannot create .velora directory: %v", err)
	}

	dbPath := filepath.Join(dotDir, "velora.sqlite")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		util.Fatalf("error opening database: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS activities (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME NOT NULL CHECK(timestamp = CAST(timestamp AS INTEGER)),
		duration INTEGER NOT NULL,
		duration_total INTEGER NOT NULL,
		sport TEXT CHECK (sport IN ('running', 'cycling', 'swimming')) NOT NULL,
		distance INTEGER NOT NULL,
		vertical_gain INTEGER,
		notes TEXT,
		was_recommended BOOLEAN NOT NULL DEFAULT FALSE,
		segments TEXT
	)`)
	if err != nil {
		return nil, fmt.Errorf("error creating activities table: %v", err)
	}

	return db, nil
}

func InsertActivity(db *sql.DB, activity activity) error {
	verticalGain := int64(activity.a.VerticalGain)
	if verticalGain == 0 {
		verticalGain = sql.NullInt64{Valid: false}.Int64
	}

	segments, err := json.Marshal(activity.a.Segments)
	if err != nil {
		return fmt.Errorf("error marshalling segments: %v", err)
	}

	_, err = db.Exec(`INSERT INTO activities (timestamp, duration, duration_total, sport, distance, vertical_gain, notes, was_recommended, segments) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		activity.a.Time.Unix(), activity.a.Duration, activity.a.DurationTotal, activity.a.Sport.String(), activity.a.Distance, verticalGain, activity.a.Notes, activity.a.WasRecommended, segments)
	return err
}
