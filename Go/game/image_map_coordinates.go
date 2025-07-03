package game

// ImageMapTerritoryCoordinates contains the actual pixel coordinates for territories
// on the high-resolution Axis & Allies map image (scaled to 2400x1200)
var ImageMapTerritoryCoordinates = map[string]struct {
	X int
	Y int
}{
	// North America
	"Eastern US":      {X: 480, Y: 550},
	"Western US":      {X: 320, Y: 520},
	"Alaska":          {X: 120, Y: 280},
	"Western Canada":  {X: 280, Y: 380},
	"Eastern Canada":  {X: 420, Y: 350},
	"Mexico":          {X: 360, Y: 650},
	"Central America": {X: 400, Y: 720},

	// South America
	"Venezuela": {X: 520, Y: 780},
	"Brazil":    {X: 620, Y: 900},
	"Argentina": {X: 560, Y: 1050},

	// Europe
	"Britain":         {X: 1100, Y: 350},
	"Western Europe":  {X: 1120, Y: 420},
	"Southern Europe": {X: 1180, Y: 480},
	"Germany":         {X: 1180, Y: 380},
	"Eastern Europe":  {X: 1260, Y: 380},
	"Norway Finland":  {X: 1180, Y: 280},

	// USSR
	"Karelia":          {X: 1320, Y: 280},
	"Russia":           {X: 1380, Y: 350},
	"Caucases":         {X: 1420, Y: 480},
	"Ukraine":          {X: 1320, Y: 420},
	"Kazakstan":        {X: 1480, Y: 420},
	"Novosibirsk":      {X: 1580, Y: 350},
	"Evenki":           {X: 1680, Y: 280},
	"Yakutsk SSR":      {X: 1780, Y: 280},
	"Soviet Far East":  {X: 1920, Y: 350},

	// Africa
	"Algeria":            {X: 1080, Y: 580},
	"Libya":              {X: 1200, Y: 580},
	"Egypt":              {X: 1320, Y: 580},
	"French West Africa": {X: 1020, Y: 680},
	"French East Africa": {X: 1140, Y: 680},
	"Congo":              {X: 1200, Y: 780},
	"South Africa":       {X: 1280, Y: 920},
	"Madagascar":         {X: 1420, Y: 880},

	// Middle East
	"Turkey":      {X: 1360, Y: 480},
	"Syria":       {X: 1400, Y: 520},
	"Iraq":        {X: 1460, Y: 520},
	"Iran":        {X: 1520, Y: 520},
	"Arabia":      {X: 1420, Y: 620},
	"Afghanistan": {X: 1580, Y: 480},

	// Asia
	"India":                    {X: 1620, Y: 580},
	"Burma and South East Asia": {X: 1720, Y: 620},
	"Sinkiang Western China":   {X: 1680, Y: 450},
	"Mongolia":                 {X: 1780, Y: 420},
	"Central China":            {X: 1820, Y: 520},
	"Kwantung Eastern China":   {X: 1920, Y: 520},
	"Manchuria":                {X: 1920, Y: 420},
	"Japan":                    {X: 2080, Y: 480},

	// Pacific Islands
	"Hawaii":           {X: 680, Y: 600},
	"Midway Island":    {X: 580, Y: 550},
	"Wake Island":      {X: 2200, Y: 600},
	"Philipines":       {X: 1980, Y: 680},
	"Caroline Islands": {X: 2120, Y: 720},
	"Solomon Islands":  {X: 2220, Y: 820},
	"New Guinea":       {X: 2080, Y: 820},
	"East Indies":      {X: 1920, Y: 780},
	"Borneo":           {X: 1860, Y: 720},
	"Okinawa":          {X: 2020, Y: 580},

	// Australia & New Zealand
	"Australia":   {X: 2020, Y: 980},
	"New Zealand": {X: 2280, Y: 1080},
}

// ImageMapPolygons defines simplified clickable regions for territories
// These are rough polygons that encompass each territory
var ImageMapPolygons = map[string]string{
	// North America
	"Eastern US": "M 440,500 L 520,500 L 520,600 L 440,600 Z",
	"Western US": "M 280,480 L 360,480 L 360,560 L 280,560 Z",
	"Alaska": "M 80,240 L 160,240 L 160,320 L 80,320 Z",
	"Western Canada": "M 240,340 L 320,340 L 320,420 L 240,420 Z",
	"Eastern Canada": "M 380,310 L 460,310 L 460,390 L 380,390 Z",
	"Mexico": "M 320,610 L 400,610 L 400,690 L 320,690 Z",
	"Central America": "M 360,680 L 440,680 L 440,760 L 360,760 Z",

	// South America
	"Venezuela": "M 480,740 L 560,740 L 560,820 L 480,820 Z",
	"Brazil": "M 580,860 L 660,860 L 660,940 L 580,940 Z",
	"Argentina": "M 520,1010 L 600,1010 L 600,1090 L 520,1090 Z",

	// Europe
	"Britain": "M 1060,310 L 1140,310 L 1140,390 L 1060,390 Z",
	"Western Europe": "M 1080,380 L 1160,380 L 1160,460 L 1080,460 Z",
	"Southern Europe": "M 1140,440 L 1220,440 L 1220,520 L 1140,520 Z",
	"Germany": "M 1140,340 L 1220,340 L 1220,420 L 1140,420 Z",
	"Eastern Europe": "M 1220,340 L 1300,340 L 1300,420 L 1220,420 Z",
	"Norway Finland": "M 1140,240 L 1220,240 L 1220,320 L 1140,320 Z",

	// Africa (simplified)
	"Algeria": "M 1040,540 L 1120,540 L 1120,620 L 1040,620 Z",
	"Libya": "M 1160,540 L 1240,540 L 1240,620 L 1160,620 Z",
	"Egypt": "M 1280,540 L 1360,540 L 1360,620 L 1280,620 Z",
	"Congo": "M 1160,740 L 1240,740 L 1240,820 L 1160,820 Z",
	"South Africa": "M 1240,880 L 1320,880 L 1320,960 L 1240,960 Z",

	// Asia (simplified)
	"India": "M 1580,540 L 1660,540 L 1660,620 L 1580,620 Z",
	"Japan": "M 2040,440 L 2120,440 L 2120,520 L 2040,520 Z",
	"Australia": "M 1980,940 L 2060,940 L 2060,1020 L 1980,1020 Z",
}