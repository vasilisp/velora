package profile

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/vasilisp/velora/internal/db"
)

type SkeletonDay struct {
	Weekday     string       `json:"weekday" jsonschema:"description=The day of the week (Monday, Tuesday, etc.)"`
	Sport       string       `json:"sport" jsonschema:"description=The type of sport (running, cycling, swimming)"`
	DistanceMin int          `json:"distance_min" jsonschema:"description=Minimum suggested distance in meters"`
	Segments    []db.Segment `json:"segments" jsonschema:"description=The segments of the workout"`
}

type SkeletonConflict struct {
	Weekday string `json:"weekday" jsonschema:"description=The day of the week (Monday, Tuesday, etc.)"`
	Sport   string `json:"sport" jsonschema:"description=The type of sport (running, cycling, swimming) not allowed on the specific  day"`
}

type Skeleton struct {
	Sports    []string           `json:"sports" jsonschema:"description=The sports that are allowed in the plan (running, cycling, swimming)"`
	Days      []SkeletonDay      `json:"days" jsonschema:"description=The days of the week and their suggested workouts"`
	Conflicts []SkeletonConflict `json:"conflicts" jsonschema:"description=The days of the week and the sports that are not allowed on that day"`
}

func skeletonPath() string {
	return filepath.Join(os.Getenv("HOME"), ".velora", "skeleton.json")
}

func WriteSkeleton(skeleton *Skeleton) error {
	json, err := json.MarshalIndent(skeleton, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(skeletonPath(), json, 0644)
}

func ReadSkeleton() (*Skeleton, error) {
	jsonBytes, err := os.ReadFile(skeletonPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &Skeleton{
				Sports:    []string{},
				Days:      []SkeletonDay{},
				Conflicts: []SkeletonConflict{},
			}, nil
		}

		return nil, err
	}

	var skeleton Skeleton
	if err := json.Unmarshal(jsonBytes, &skeleton); err != nil {
		return nil, err
	}

	return &skeleton, nil
}
