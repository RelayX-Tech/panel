package settings

import (
	"github.com/relayx-tech/panel/internal/logic/shared"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/types"
)

func toGeoFile(f model.GeoFile) types.GeoFile {
	return types.GeoFile{
		Id: f.ID, Kind: f.Kind, Name: f.Name, Version: f.Version, SizeBytes: f.SizeBytes,
		CategoriesCount: f.CategoriesCount, Checksum: f.Checksum, UpdatedAt: shared.TimeString(f.UpdatedAt),
		Status: f.Status,
	}
}
