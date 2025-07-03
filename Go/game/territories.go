package game

type TerritoryCoordinates struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	IsLand bool    `json:"is_land"`
}

var TerritoryPositions map[string]TerritoryCoordinates

func init() {
	// Use the full territory positions
	TerritoryPositions = FullTerritoryPositions
}

type PlayerColor struct {
	Primary   string `json:"primary"`
	Secondary string `json:"secondary"`
}

var PlayerColors = map[string]PlayerColor{
	"USA":     {Primary: "#1e3a8a", Secondary: "#3b82f6"},
	"USSR":    {Primary: "#991b1b", Secondary: "#dc2626"},
	"Germany": {Primary: "#4b5563", Secondary: "#6b7280"},
	"Japan":   {Primary: "#f59e0b", Secondary: "#fbbf24"},
	"UK":      {Primary: "#059669", Secondary: "#10b981"},
	"Britain": {Primary: "#059669", Secondary: "#10b981"},
	"default": {Primary: "#6b7280", Secondary: "#9ca3af"},
}