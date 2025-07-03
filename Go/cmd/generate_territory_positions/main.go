package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/boardgame/Go/game"
	"github.com/boardgame/Go/parsing"
)

// TerritoryPosition represents a territory's position on the map
type TerritoryPosition struct {
	Name   string  `json:"name"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	IsLand bool    `json:"is_land"`
}

// Define positions for all territories based on real-world geography
// Using a 2000x1000 coordinate system
var worldPositions = map[string]TerritoryPosition{
	// Europe
	"Germany":         {"Germany", 1000, 300, true},
	"Norway Finland":  {"Norway Finland", 1020, 200, true},
	"Britain":         {"Britain", 920, 280, true},
	"Western Europe":  {"Western Europe", 950, 340, true},
	"Southern Europe": {"Southern Europe", 1020, 380, true},
	"Spain":           {"Spain", 900, 380, true},
	"Eastern Europe":  {"Eastern Europe", 1080, 320, true},
	
	// Soviet Union
	"Karelia":        {"Karelia", 1100, 220, true},
	"Russia":         {"Russia", 1200, 260, true},
	"Ukraine":        {"Ukraine", 1120, 360, true},
	"Caucases":       {"Caucases", 1180, 400, true},
	"Kazakstan":      {"Kazakstan", 1300, 350, true},
	"Yakutsk SSR":    {"Yakutsk SSR", 1600, 180, true},
	"Evenki":         {"Evenki", 1450, 200, true},
	"Novosibirsk":    {"Novosibirsk", 1400, 280, true},
	"Soviet Far East": {"Soviet Far East", 1700, 240, true},
	
	// Asia
	"Mongolia":                   {"Mongolia", 1500, 350, true},
	"Afghanistan":                {"Afghanistan", 1280, 420, true},
	"Manchuria":                  {"Manchuria", 1620, 340, true},
	"Sinkiang Western China":     {"Sinkiang Western China", 1400, 400, true},
	"Kwantung Eastern China":     {"Kwantung Eastern China", 1550, 420, true},
	"Central China":              {"Central China", 1480, 440, true},
	"Japan":                      {"Japan", 1720, 380, true},
	"Burma and South East Asia":  {"Burma and South East Asia", 1420, 520, true},
	"India":                      {"India", 1340, 480, true},
	"Okinawa":                    {"Okinawa", 1660, 460, true},
	
	// Middle East
	"Turkey":    {"Turkey", 1120, 420, true},
	"Palestine": {"Palestine", 1140, 460, true},
	"Syria":     {"Syria", 1160, 440, true},
	"Iran":      {"Iran", 1220, 440, true},
	"Iraq":      {"Iraq", 1180, 460, true},
	"Arabia":    {"Arabia", 1200, 500, true},
	
	// Africa
	"Algeria":               {"Algeria", 960, 440, true},
	"Libya":                 {"Libya", 1040, 460, true},
	"Egypt":                 {"Egypt", 1100, 480, true},
	"French West Africa":    {"French West Africa", 920, 520, true},
	"French East Africa":    {"French East Africa", 1000, 540, true},
	"Ethiopia":              {"Ethiopia", 1160, 540, true},
	"Kenya":                 {"Kenya", 1140, 600, true},
	"Congo":                 {"Congo", 1060, 620, true},
	"South Africa":          {"South Africa", 1080, 740, true},
	"Madagascar":            {"Madagascar", 1200, 700, true},
	"Mozambique":            {"Mozambique", 1140, 680, true},
	"Angola":                {"Angola", 1020, 660, true},
	
	// Pacific Islands
	"East Indies":      {"East Indies", 1560, 600, true},
	"Philipines":       {"Philipines", 1620, 520, true},
	"New Guinea":       {"New Guinea", 1680, 600, true},
	"Borneo":           {"Borneo", 1540, 560, true},
	"Caroline Islands": {"Caroline Islands", 1740, 540, true},
	"Solomon Islands":  {"Solomon Islands", 1780, 620, true},
	"Wake Island":      {"Wake Island", 1840, 480, true},
	"Midway Island":    {"Midway Island", 1900, 440, true},
	"Hawaii":           {"Hawaii", 1960, 500, true},
	
	// Australia/New Zealand
	"Australia":   {"Australia", 1680, 760, true},
	"New Zealand": {"New Zealand", 1820, 840, true},
	
	// North America
	"Alaska":         {"Alaska", 100, 200, true},
	"Western Canada": {"Western Canada", 200, 260, true},
	"Eastern Canada": {"Eastern Canada", 340, 280, true},
	"Western US":     {"Western US", 240, 360, true},
	"Eastern US":     {"Eastern US", 360, 380, true},
	"Mexico":         {"Mexico", 260, 440, true},
	
	// Central/South America
	"Central America": {"Central America", 280, 500, true},
	"Colombia":        {"Colombia", 320, 560, true},
	"Venezuela":       {"Venezuela", 380, 580, true},
	"Brazil":          {"Brazil", 440, 660, true},
	"Peru":            {"Peru", 340, 680, true},
	"Chile":           {"Chile", 340, 780, true},
	"Argentina":       {"Argentina", 380, 820, true},
	"West Indies":     {"West Indies", 360, 480, true},
	
	// Atlantic Ocean
	"North Sea":                        {"North Sea", 980, 240, false},
	"Baltic Sea":                       {"Baltic Sea", 1040, 260, false},
	"Eastern Atlantic":                 {"Eastern Atlantic", 860, 400, false},
	"Western Mediteranian":             {"Western Mediteranian", 980, 400, false},
	"Eastern Mediteranian":             {"Eastern Mediteranian", 1080, 420, false},
	"Black Sea":                        {"Black Sea", 1140, 380, false},
	"North Central Atlantic":           {"North Central Atlantic", 700, 400, false},
	"Central south Atlantic":           {"Central south Atlantic", 600, 660, false},
	"South West African Atlantic":      {"South West African Atlantic", 900, 720, false},
	"West African Atlantic":            {"West African Atlantic", 800, 500, false},
	"Southern West African Atlantic":   {"Southern West African Atlantic", 840, 600, false},
	"South Argentinian Antarctic Ocean": {"South Argentinian Antarctic Ocean", 400, 900, false},
	"Antarctic Ocean":                  {"Antarctic Ocean", 700, 920, false},
	"Argentine Atlantic":               {"Argentine Atlantic", 480, 800, false},
	"East Brazil Atlantic":             {"East Brazil Atlantic", 520, 640, false},
	"Canadian Atlantic":                {"Canadian Atlantic", 500, 300, false},
	"Karelia Sea":                      {"Karelia Sea", 1060, 160, false},
	"North Eastern Brazil Atlantic":    {"North Eastern Brazil Atlantic", 560, 560, false},
	"Eastern USA Atlantic":             {"Eastern USA Atlantic", 440, 380, false},
	"Carribean Sea":                    {"Carribean Sea", 400, 480, false},
	
	// Indian Ocean
	"Red Sea":                                 {"Red Sea", 1160, 500, false},
	"Madagascar Sea":                          {"Madagascar Sea", 1240, 720, false},
	"South West Indian Ocean":                 {"South West Indian Ocean", 1160, 800, false},
	"Western Indian Ocean":                    {"Western Indian Ocean", 1200, 640, false},
	"Central Indian Ocean":                    {"Central Indian Ocean", 1320, 700, false},
	"Southern Indian Ocean":                   {"Southern Indian Ocean", 1280, 880, false},
	"North Central Indian Ocean":              {"North Central Indian Ocean", 1280, 600, false},
	"Indian Ocean":                            {"Indian Ocean", 1340, 560, false},
	"North East Indian Ocean":                 {"North East Indian Ocean", 1420, 620, false},
	"Bay of Bengal":                           {"Bay of Bengal", 1440, 540, false},
	"Antarctic Ocean southwest of Australia":  {"Antarctic Ocean southwest of Australia", 1500, 900, false},
	"South Australian Ocean":                  {"South Australian Ocean", 1680, 860, false},
	"Eastern Indian Ocean":                    {"Eastern Indian Ocean", 1480, 740, false},
	"Western Australian Ocean":                {"Western Australian Ocean", 1560, 720, false},
	"East Indies Ocean":                       {"East Indies Ocean", 1520, 640, false},
	
	// Pacific Ocean
	"South China Sea":       {"South China Sea", 1540, 500, false},
	"Borneo Sea":            {"Borneo Sea", 1580, 580, false},
	"East Australia Sea":    {"East Australia Sea", 1780, 780, false},
	"New Guinea Sea":        {"New Guinea Sea", 1720, 620, false},
	"Solomon Island Sea":    {"Solomon Island Sea", 1820, 660, false},
	"Wake Island Sea":       {"Wake Island Sea", 1880, 500, false},
	"Caroline Islands Sea":  {"Caroline Islands Sea", 1780, 560, false},
	"Okinawa Sea":           {"Okinawa Sea", 1680, 420, false},
	"Philipines Sea":        {"Philipines Sea", 1660, 540, false},
	"East China Sea":        {"East China Sea", 1600, 400, false},
	"Sea of Japan":          {"Sea of Japan", 1680, 360, false},
	"North Central Pacific": {"North Central Pacific", 1800, 400, false},
	"Central Pacific":       {"Central Pacific", 1900, 600, false},
	"Western US Pacific":    {"Western US Pacific", 160, 400, false},
	"Canadian Pacific":      {"Canadian Pacific", 120, 300, false},
	"Alaskan Pacific":       {"Alaskan Pacific", 60, 240, false},
	"North West Pacific":    {"North West Pacific", 1760, 300, false},
	"Midway Island sea":     {"Midway Island sea", 1940, 460, false},
	"New Zealand Sea":       {"New Zealand Sea", 1860, 820, false},
	"Hawaian Sea":           {"Hawaian Sea", 2000, 520, false},
	"Mexican Pacific":       {"Mexican Pacific", 200, 480, false},
	"South East Pacific":    {"South East Pacific", 280, 740, false},
	"South Pacific":         {"South Pacific", 1960, 760, false},
}

func main() {
	// Load the aaa.gdf file
	file, err := os.Open("../aaa.gdf")
	if err != nil {
		log.Fatalf("Failed to open aaa.gdf: %v", err)
	}
	defer file.Close()

	parser := parsing.NewGameParser(file)
	gameState, err := parser.Load()
	if err != nil {
		log.Fatalf("Failed to parse game: %v", err)
	}

	// Create game to get territory data
	g, err := game.NewGame(gameState)
	if err != nil {
		log.Fatalf("Failed to create game: %v", err)
	}

	// Generate territory positions map
	territoryPositions := make(map[string]game.TerritoryCoordinates)
	
	for _, territory := range g.Board {
		if pos, ok := worldPositions[territory.Name]; ok {
			territoryPositions[territory.Name] = game.TerritoryCoordinates{
				X:      pos.X,
				Y:      pos.Y,
				IsLand: pos.IsLand,
			}
		} else {
			// Default position for missing territories
			fmt.Printf("Warning: No position defined for territory: %s\n", territory.Name)
			territoryPositions[territory.Name] = game.TerritoryCoordinates{
				X:      1000,
				Y:      500,
				IsLand: strings.Contains(strings.ToLower(territory.Name), "sea") || 
				        strings.Contains(strings.ToLower(territory.Name), "ocean") ||
				        strings.Contains(strings.ToLower(territory.Name), "atlantic") ||
				        strings.Contains(strings.ToLower(territory.Name), "pacific"),
			}
		}
	}

	// Output as Go code
	fmt.Println("// TerritoryPositions contains map coordinates for each territory")
	fmt.Println("var TerritoryPositions = map[string]TerritoryCoordinates{")
	
	for name, coords := range territoryPositions {
		fmt.Printf("\t%q: {X: %.0f, Y: %.0f, IsLand: %t},\n", 
			name, coords.X, coords.Y, coords.IsLand)
	}
	
	fmt.Println("}")

	// Also save as JSON for the web interface
	jsonData, err := json.MarshalIndent(territoryPositions, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal positions: %v", err)
	}

	err = os.WriteFile("territory_positions.json", jsonData, 0644)
	if err != nil {
		log.Fatalf("Failed to write positions file: %v", err)
	}

	fmt.Println("\nTerritory positions saved to territory_positions.json")
	fmt.Printf("Total territories: %d\n", len(territoryPositions))
}