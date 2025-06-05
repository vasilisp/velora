package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
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
	Repeat   int `json:"repeat" jsonschema_description:"The number of times to repeat the segment. Can be 1."`
	Distance int `json:"distance" jsonschema_description:"The planned distance in meters"`
	Zone     int `json:"zone" jsonschema_description:"The planned zone (1-5)"`
}

type ActivityUnsafe struct {
	Time           time.Time `json:"time"`
	Duration       int       `json:"duration"`
	DurationTotal  int       `json:"duration_total,omitempty"`
	Distance       int       `json:"distance"`
	Sport          string    `json:"sport"`
	VerticalGain   int       `json:"vertical_gain"`
	Notes          string    `json:"notes"`
	WasRecommended bool      `json:"was_recommended"`
	Segments       []Segment `json:"segments" jsonschema_description:"The segments of the activity; should be empty for non-structured activities"`
}

func outputSegmentsTo(w io.Writer, segments []Segment) {
	for i, segment := range segments {
		if i > 0 {
			fmt.Fprint(w, ", ")
		}
		fmt.Fprintf(w, "  - Repeat: %d, Distance: %d, Zone: %d\n", segment.Repeat, segment.Distance, segment.Zone)
	}
}

func (a *ActivityUnsafe) OutputTo(w io.Writer) {
	util.Assert(a != nil, "OutputTo nil activity")

	fmt.Fprintf(w, "Date: %s\nSport: %s\nTime: %s\nDistance: %s\nVertical Gain: %dm\nNotes: %s\n",
		a.Time.Format("Jan 2, 15:04"),
		a.Sport,
		util.FormatDuration(a.Duration),
		util.FormatDistance(a.Distance),
		a.VerticalGain,
		extra.SanitizeOutputString(a.Notes, true),
	)
	outputSegmentsTo(w, a.Segments)
}

type activity struct {
	a     ActivityUnsafe
	sport Sport
}

func (a ActivityUnsafe) ToActivity() (activity, error) {
	if a.DurationTotal <= 0 {
		a.DurationTotal = a.Duration
	}

	sport, err := SportFromString(a.Sport)
	if err != nil {
		return activity{}, err
	}

	if a.Duration <= 0 {
		err = fmt.Errorf("duration must be positive")
	}

	if a.Distance <= 0 {
		err = fmt.Errorf("distance must be positive")
	}

	if a.VerticalGain < 0 {
		err = fmt.Errorf("verticalGain must be non-negative")
	}

	return activity{a: a, sport: sport}, err
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
		var activity ActivityUnsafe
		var verticalGain sql.NullInt64
		var segmentsBytes []byte

		err := rows.Scan(&activity.Time, &activity.Duration, &activity.DurationTotal, &activity.Sport, &activity.Distance, &verticalGain, &activity.Notes, &activity.WasRecommended, &segmentsBytes)
		if err != nil {
			return nil, fmt.Errorf("error scanning activity: %v", err)
		}

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
		activity.a.Time.Unix(), activity.a.Duration, activity.a.DurationTotal, activity.sport.String(), activity.a.Distance, verticalGain, activity.a.Notes, activity.a.WasRecommended, segments)
	return err
}
