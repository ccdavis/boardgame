#pragma once

#include "parser.h"
#include "token.h"



#include<string>
#include<map>
#include<vector>
#include<set>
#include<memory>


// We need this to store the partially loaded game info in
// a somewhat raw form, so we can do basic validation
// on it, sort of like building up a symbol table and checking for basic problems
// in a programming language.
struct GameState {

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

typedef std::shared_ptr<GameState> game_state_ptr;

class GameParser:public ScriptParser {
public:

    GameParser(const std::string & game_script_file);

private:


    // The plan is to have the parser return fairly generic data types which the
    // actual game engine will ingest and use to
    // construct the real game data structures.

public:
    game_state_ptr load();

private:
    void players(GameState &game);
    void turn(GameState &game);
    void territories(GameState &game);
    void game_map(GameState &game);
    void units(GameState &game);
    void containers(GameState &game);
    void placement(GameState &game);

    std::string parse_territory_name();
};
