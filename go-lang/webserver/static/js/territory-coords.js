/**
 * Territory coordinate mapping for clickable map regions
 * Coordinates are percentages of image dimensions (0-100)
 * Format: { x: %, y: %, radius: % } for circular click regions
 *
 * Image dimensions reference: ~960x540 pixels
 * Coordinates derived from visual analysis of hires_aaa_map.jpg
 */

const TERRITORY_COORDS = {
    // EUROPE
    "Germany": { x: 48.5, y: 23.5, radius: 2.5 },
    "Britain": { x: 44.5, y: 20.0, radius: 2.0 },
    "Western Europe": { x: 46.0, y: 26.0, radius: 2.5 },
    "Southern Europe": { x: 49.5, y: 28.5, radius: 2.5 },
    "Spain": { x: 44.0, y: 29.0, radius: 2.0 },
    "Eastern Europe": { x: 52.0, y: 24.5, radius: 2.5 },
    "Ukraine": { x: 54.5, y: 24.0, radius: 2.5 },
    "Norway Finland": { x: 50.5, y: 18.5, radius: 3.0 },
    "Karelia": { x: 54.0, y: 19.5, radius: 2.5 },
    "Russia": { x: 58.5, y: 20.5, radius: 3.0 },
    "Caucases": { x: 57.0, y: 27.0, radius: 2.5 },

    // SOVIET TERRITORIES
    "Kazakstan": { x: 61.5, y: 27.5, radius: 3.0 },
    "Novosibirsk": { x: 65.0, y: 22.0, radius: 2.5 },
    "Evenki": { x: 68.0, y: 18.5, radius: 2.5 },
    "Yakutsk SSR": { x: 72.0, y: 17.0, radius: 3.0 },
    "Soviet Far East": { x: 75.5, y: 19.5, radius: 2.5 },

    // ASIA
    "Mongolia": { x: 68.5, y: 25.0, radius: 2.5 },
    "Afghanistan": { x: 62.0, y: 31.0, radius: 2.0 },
    "Sinkiang Western China": { x: 65.5, y: 28.5, radius: 2.5 },
    "Manchuria": { x: 72.5, y: 23.0, radius: 2.5 },
    "Kwantung Eastern China": { x: 71.0, y: 27.5, radius: 2.0 },
    "Central China": { x: 68.5, y: 29.0, radius: 2.5 },
    "Japan": { x: 76.0, y: 27.0, radius: 2.0 },
    "Okinawa": { x: 74.5, y: 30.0, radius: 1.5 },

    // MIDDLE EAST
    "Turkey": { x: 54.5, y: 29.5, radius: 2.0 },
    "Syria": { x: 56.0, y: 32.0, radius: 1.5 },
    "Iraq": { x: 58.0, y: 32.5, radius: 2.0 },
    "Iran": { x: 60.5, y: 31.5, radius: 2.5 },
    "Palestine": { x: 55.5, y: 33.5, radius: 1.5 },
    "Arabia": { x: 58.0, y: 36.0, radius: 2.5 },

    // SOUTHEAST ASIA & PACIFIC ISLANDS
    "India": { x: 63.5, y: 35.0, radius: 2.5 },
    "Burma and South East Asia": { x: 67.5, y: 34.0, radius: 2.5 },
    "East Indies": { x: 69.0, y: 42.0, radius: 2.0 },
    "Borneo": { x: 70.5, y: 40.0, radius: 1.5 },
    "Philipines": { x: 73.0, y: 36.0, radius: 1.5 },
    "New Guinea": { x: 76.5, y: 43.0, radius: 2.0 },
    "Australia": { x: 77.5, y: 51.5, radius: 4.0 },
    "New Zealand": { x: 82.0, y: 57.0, radius: 1.5 },
    "Caroline Islands": { x: 78.0, y: 40.0, radius: 1.5 },
    "Solomon Islands": { x: 80.5, y: 43.5, radius: 1.5 },

    // NORTH AMERICA
    "Alaska": { x: 12.0, y: 14.5, radius: 3.0 },
    "Western Canada": { x: 17.0, y: 18.0, radius: 3.5 },
    "Eastern Canada": { x: 23.5, y: 19.5, radius: 3.0 },
    "Western US": { x: 17.5, y: 26.0, radius: 3.0 },
    "Eastern US": { x: 23.5, y: 27.5, radius: 3.0 },
    "Mexico": { x: 19.0, y: 32.5, radius: 2.5 },
    "Central America": { x: 20.5, y: 37.0, radius: 2.0 },

    // PACIFIC ISLANDS (US)
    "Hawaii": { x: 16.0, y: 35.0, radius: 1.5 },
    "Midway Island": { x: 22.0, y: 30.0, radius: 1.0 },
    "Wake Island": { x: 26.0, y: 32.0, radius: 1.0 },

    // SOUTH AMERICA
    "Colombia": { x: 23.5, y: 42.0, radius: 2.0 },
    "Venezuela": { x: 26.0, y: 41.0, radius: 2.0 },
    "Brazil": { x: 29.5, y: 46.5, radius: 4.0 },
    "Peru": { x: 24.5, y: 47.0, radius: 2.0 },
    "Chile": { x: 25.0, y: 53.5, radius: 2.5 },
    "Argentina": { x: 27.5, y: 54.0, radius: 2.5 },
    "West Indies": { x: 25.0, y: 36.0, radius: 1.5 },

    // AFRICA
    "Algeria": { x: 46.5, y: 33.5, radius: 2.5 },
    "Libya": { x: 50.0, y: 35.0, radius: 2.5 },
    "Egypt": { x: 53.0, y: 35.5, radius: 2.5 },
    "French West Africa": { x: 45.0, y: 38.5, radius: 3.0 },
    "French East Africa": { x: 50.5, y: 40.0, radius: 2.5 },
    "Ethiopia": { x: 54.5, y: 39.5, radius: 2.0 },
    "Kenya": { x: 54.5, y: 43.0, radius: 2.0 },
    "Congo": { x: 51.0, y: 44.5, radius: 2.5 },
    "Angola": { x: 50.0, y: 48.0, radius: 2.0 },
    "Mozambique": { x: 54.0, y: 49.0, radius: 2.0 },
    "South Africa": { x: 52.0, y: 52.5, radius: 2.5 },
    "Madagascar": { x: 56.5, y: 50.0, radius: 1.5 },

    // EUROPEAN SEAS
    "North Sea": { x: 47.0, y: 20.0, radius: 2.0 },
    "Baltic Sea": { x: 50.5, y: 21.0, radius: 1.5 },
    "Karelia Sea": { x: 54.0, y: 16.5, radius: 2.0 },
    "Black Sea": { x: 54.5, y: 27.5, radius: 1.5 },
    "Western Mediteranian": { x: 47.5, y: 30.5, radius: 2.0 },
    "Eastern Mediteranian": { x: 52.5, y: 32.5, radius: 2.0 },
    "Red Sea": { x: 56.0, y: 36.5, radius: 1.5 },

    // ATLANTIC OCEAN
    "Eastern Atlantic": { x: 42.5, y: 28.0, radius: 2.5 },
    "North Central Atlantic": { x: 38.0, y: 26.0, radius: 2.5 },
    "Canadian Atlantic": { x: 30.0, y: 22.0, radius: 2.5 },
    "Eastern USA Atlantic": { x: 28.0, y: 28.0, radius: 2.0 },
    "Carribean Sea": { x: 25.0, y: 38.0, radius: 2.0 },
    "Central south Atlantic": { x: 38.0, y: 40.0, radius: 3.0 },
    "West African Atlantic": { x: 42.0, y: 37.0, radius: 2.0 },
    "Southern West African Atlantic": { x: 42.0, y: 43.0, radius: 2.0 },
    "South West African Atlantic": { x: 44.0, y: 48.0, radius: 2.5 },
    "East Brazil Atlantic": { x: 32.0, y: 44.0, radius: 2.5 },
    "North Eastern Brazil Atlantic": { x: 30.0, y: 40.0, radius: 2.0 },
    "Argentine Atlantic": { x: 32.0, y: 52.0, radius: 2.5 },
    "South Argentinian Antarctic Ocean": { x: 28.0, y: 58.0, radius: 3.0 },
    "Antarctic Ocean": { x: 48.0, y: 60.0, radius: 5.0 },

    // INDIAN OCEAN
    "Western Indian Ocean": { x: 57.0, y: 42.0, radius: 2.5 },
    "Central Indian Ocean": { x: 61.0, y: 44.0, radius: 2.5 },
    "Indian Ocean": { x: 61.0, y: 39.0, radius: 2.5 },
    "North Central Indian Ocean": { x: 63.0, y: 41.0, radius: 2.0 },
    "North East Indian Ocean": { x: 65.0, y: 39.0, radius: 2.0 },
    "Bay of Bengal": { x: 66.5, y: 37.0, radius: 2.0 },
    "South West Indian Ocean": { x: 56.0, y: 48.0, radius: 2.5 },
    "Southern Indian Ocean": { x: 64.0, y: 52.0, radius: 3.0 },
    "Madagascar Sea": { x: 58.0, y: 50.0, radius: 2.0 },
    "Western Australian Ocean": { x: 70.0, y: 50.0, radius: 2.5 },
    "Eastern Indian Ocean": { x: 68.0, y: 47.0, radius: 2.5 },
    "South Australian Ocean": { x: 75.0, y: 54.0, radius: 2.5 },
    "Antarctic Ocean southwest of Australia": { x: 72.0, y: 58.0, radius: 3.0 },

    // PACIFIC OCEAN
    "East Indies Ocean": { x: 69.5, y: 44.5, radius: 2.0 },
    "Borneo Sea": { x: 71.5, y: 41.5, radius: 1.5 },
    "South China Sea": { x: 70.0, y: 36.5, radius: 2.0 },
    "East China Sea": { x: 72.5, y: 30.0, radius: 2.0 },
    "Sea of Japan": { x: 75.0, y: 25.0, radius: 2.0 },
    "Okinawa Sea": { x: 74.0, y: 32.0, radius: 1.5 },
    "Philipines Sea": { x: 74.5, y: 37.0, radius: 2.0 },
    "New Guinea Sea": { x: 77.5, y: 44.0, radius: 2.0 },
    "Solomon Island Sea": { x: 80.0, y: 45.0, radius: 2.0 },
    "East Australia Sea": { x: 80.0, y: 51.0, radius: 2.5 },
    "New Zealand Sea": { x: 81.5, y: 54.5, radius: 2.0 },
    "Caroline Islands Sea": { x: 78.5, y: 41.5, radius: 2.0 },
    "North West Pacific": { x: 77.5, y: 22.0, radius: 3.0 },
    "Wake Island Sea": { x: 81.0, y: 32.0, radius: 2.0 },
    "Midway Island sea": { x: 85.0, y: 30.0, radius: 2.0 },
    "Hawaian Sea": { x: 88.0, y: 35.0, radius: 2.5 },
    "Central Pacific": { x: 84.0, y: 40.0, radius: 3.0 },
    "North Central Pacific": { x: 88.0, y: 28.0, radius: 3.0 },
    "South Pacific": { x: 86.0, y: 50.0, radius: 4.0 },
    "Alaskan Pacific": { x: 14.0, y: 18.0, radius: 2.5 },
    "Canadian Pacific": { x: 18.0, y: 22.0, radius: 2.5 },
    "Western US Pacific": { x: 20.0, y: 28.0, radius: 2.5 },
    "Mexican Pacific": { x: 18.5, y: 35.0, radius: 2.5 },
    "South East Pacific": { x: 24.0, y: 50.0, radius: 3.5 }
};

// Export for use in app
window.TERRITORY_COORDS = TERRITORY_COORDS;
