package mobileapi

import "mobile_server/internal/core"

type ProfilePrefs = core.ProfilePrefs
type ProfileStore = core.ProfileStore

func NewProfileStore(path string) *ProfileStore {
	return core.NewProfileStore(path)
}
