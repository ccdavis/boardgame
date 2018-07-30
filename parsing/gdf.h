
#pragma once

#include "parser.h"
#include "types.h"
// Game Definition File parser
// Returns vectors and maps of strings and numbers
// which can be easily converted to JSON or other  common formats.
class GDF:public ScriptParser {

public:

    GDF(std::string game_file):
        ScriptParser(std::string game_file) {}

private:

// Names of players, referred to in the game map and placement sections
    std::vector<std::string> players;

    int turn;

    // key is territory name, values are map<attributes, value>
    std::map<std::string, std::map<std::string,std::string>> territories;

    // key is territory name, value is vector<connected territories>
    std::map<std::string,std::vector<std::string>> game_map;

    // key is unit name, value is map of attributes <attr-name, value>
    std::map<std::string, std::map<std::string,int>> units;

    // Key is a unit type, value is a map<unit type, number>
    std::map<std::string,std::map<std::string,int>> containers;

    // Key is a territory name, value is a map<unit type, number>
    std::map<std::string, std::map<std::string, int>> placement;



};
