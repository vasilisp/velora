package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
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

type Activity struct {
	timestamp     time.Time
	duration      int
	durationTotal int
	distance      int
	sport         Sport
}

func NewActivity(timestamp time.Time, duration int, durationTotal int, distance int, sport Sport) (*Activity, error) {
	if duration <= 0 {
		return nil, fmt.Errorf("duration must be positive")
	}
	if durationTotal <= 0 {
		return nil, fmt.Errorf("durationTotal must be positive")
	}
	if distance <= 0 {
		return nil, fmt.Errorf("distance must be positive")
	}
	if sport < Running || sport > Swimming {
		return nil, fmt.Errorf("invalid sport")
	}

	return &Activity{timestamp: timestamp, duration: duration, durationTotal: durationTotal, distance: distance, sport: sport}, nil
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
		timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		duration INTEGER NOT NULL,
		duration_total INTEGER NOT NULL,
		sport TEXT CHECK (sport IN ('running', 'cycling', 'swimming')) NOT NULL,
		distance INTEGER NOT NULL
	)`)
	if err != nil {
		return nil, fmt.Errorf("error creating activities table: %v", err)
	}
	return db, nil
}

func InsertActivity(db *sql.DB, activity Activity) error {
	_, err := db.Exec(`INSERT INTO activities (timestamp, duration, duration_total, sport, distance) VALUES (?, ?, ?, ?, ?)`,
		activity.timestamp, activity.duration, activity.durationTotal, activity.sport.String(), activity.distance)
	return err
}
