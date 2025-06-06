package profile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/vasilisp/velora/internal/util"
)

type Sport uint8

const (
	Cycling Sport = iota
	Running
)

func (s Sport) String() string {
	util.Assert(s >= Cycling && s <= Running, "invalid sport")
	return []string{"cycling", "running"}[s]
}

func ParseSport(s string) Sport {
	switch s {
	case "cycling":
		return Cycling
	case "running":
		return Running
	}

	util.Fatalf("invalid sport: %s", s)
	panic("unreachable")
}

type SportPreferences struct {
	TargetWeeklyDistance uint      `json:"target_weekly_distance"`
	TargetDistance       uint      `json:"target_distance"`
	TrainsIndoors        bool      `json:"trains_indoors"`
	TargetDistanceDate   time.Time `json:"target_distance_date,omitempty"`
}

func (sc SportPreferences) MarshalJSON() ([]byte, error) {
	type Alias SportPreferences

	if sc.TargetDistanceDate.IsZero() {
		return json.Marshal(&struct {
			*Alias
			TargetDistanceDate *string `json:"target_distance_date,omitempty"`
		}{
			Alias:              (*Alias)(&sc),
			TargetDistanceDate: nil,
		})
	}

	return json.Marshal(&struct {
		*Alias
		TargetDistanceDate string `json:"target_distance_date"`
	}{
		Alias:              (*Alias)(&sc),
		TargetDistanceDate: sc.TargetDistanceDate.Format("2006-01-02"),
	})
}

func (sc *SportPreferences) UnmarshalJSON(data []byte) error {
	type Alias SportPreferences

	aux := &struct {
		*Alias
		TargetDistanceDate string `json:"target_distance_date"`
	}{
		Alias: (*Alias)(sc),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if aux.TargetDistanceDate != "" {
		date, err := time.Parse("2006-01-02", aux.TargetDistanceDate)
		if err != nil {
			return err
		}
		sc.TargetDistanceDate = date
	}

	return nil
}

type SportMap map[Sport]SportPreferences

func (sm SportMap) MarshalJSON() ([]byte, error) {
	sportsMap := make(map[string]SportPreferences)
	for sport, constraints := range sm {
		sportsMap[sport.String()] = constraints
	}

	return json.Marshal(sportsMap)
}

func (sm *SportMap) UnmarshalJSON(data []byte) error {
	util.Assert(sm != nil, "sm is nil")
	stringMap := make(map[string]SportPreferences)

	if err := json.Unmarshal(data, &stringMap); err != nil {
		return err
	}

	*sm = make(SportMap)
	for sportStr, constraints := range stringMap {
		sport := ParseSport(sportStr)
		(*sm)[sport] = constraints
	}

	return nil
}

type Profile struct {
	Sports SportMap `json:"sports"`
	FTP    uint     `json:"ftp"`
}

func Read() Profile {
	profilePath := filepath.Join(os.Getenv("HOME"), ".velora", "prefs.json")
	profileBytes := []byte{}

	profileBytes, err := os.ReadFile(profilePath)
	if err != nil {
		util.Fatalf("error reading profile: %v\n", err)
	}

	var p Profile
	if err := json.Unmarshal(profileBytes, &p); err != nil {
		util.Fatalf("error unmarshalling profile: %v\n", err)
	}

	return p
}

func (p Profile) AllSports() []Sport {
	sports := make([]Sport, 0, len(p.Sports))
	for sport := range p.Sports {
		sports = append(sports, sport)
	}
	return sports
}
