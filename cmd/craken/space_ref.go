package main

import (
	"fmt"
	"strings"
)

func resolveSpaceRef(client apiClient, raw string) (spaceRecord, error) {
	ref := strings.TrimSpace(raw)
	if ref == "" {
		return spaceRecord{}, fmt.Errorf("space reference is required")
	}
	if strings.HasPrefix(ref, "sp_") {
		spaceID, err := validateSSHSpaceID(ref)
		if err != nil {
			return spaceRecord{}, err
		}
		return spaceRecord{ID: spaceID}, nil
	}

	spaces, err := listSpaces(client)
	if err != nil {
		return spaceRecord{}, err
	}

	for i := range spaces {
		space := spaces[i]
		if strings.TrimSpace(space.ID) == ref {
			return space, nil
		}
	}

	matches := make([]spaceRecord, 0, 1)
	for i := range spaces {
		space := spaces[i]
		if space.Name == ref {
			matches = append(matches, space)
		}
	}

	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return spaceRecord{}, fmt.Errorf("space not found: %s", ref)
	default:
		ids := make([]string, 0, len(matches))
		for i := range matches {
			ids = append(ids, matches[i].ID)
		}
		return spaceRecord{}, fmt.Errorf("space name %q is ambiguous; matches: %s", ref, strings.Join(ids, ", "))
	}
}

func listSpaces(client apiClient) ([]spaceRecord, error) {
	var response struct {
		OK     bool          `json:"ok"`
		Error  string        `json:"error"`
		Spaces []spaceRecord `json:"spaces"`
	}
	if err := client.doJSON("GET", "/api/v1/spaces", nil, &response); err != nil {
		return nil, err
	}
	return response.Spaces, nil
}
