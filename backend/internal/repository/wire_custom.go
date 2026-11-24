package repository

import "github.com/google/wire"

// CustomProviderSet holds custom repository constructors so upstream
// repository/wire.go edits remain a one-line insertion during syncs.
var CustomProviderSet = wire.NewSet(
	NewNewcomerCampaignRepository,
)
