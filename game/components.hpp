


#pragma once

#include<stdexcept>
#include <exception>
#include<string>
#include<vector>
#include<unordered_map>
#include<map>
#include<memory>

enum class terrain_t {
    land, water, both
};


terrain_t  to_terrain_t(std::string type);




class GameState;
struct Territory;
typedef int piece_id;
typedef int territory_id;
typedef int player_id;

struct Piece {
	public:
    int capacity;
    int cost;
    int attack;
    int defend;
    int movement;
    terrain_t terrain;
    std::string name;
    std::vector<std::string> can_carry;
    std::vector<piece_id> holding;
};

struct  Player {	
	public:
    std::string name;
    bool npc;
    bool active;
    std::vector<Territory*> territories;
    std::map<std::string, Piece> piece_templates;
};
		
	
struct Territory {
	public:
    std::string name;
    Player * owner;
    std::vector<piece_id> pieces;
    terrain_t terrain;
    int production;
    std::vector<Territory*> connected_to;	
};



struct Game {
    // Players and territories are unchanging
    std::vector<Territory> board;
    std::vector<Player> players;

    // piece_id is needed since pieces get added and removed
    // throughout the course of the game.
    std::map<piece_id,Piece> pieces;

    // One template per piece name
    std::map<std::string,Piece> global_piece_templates;

    Game(const GameState & loaded_game);
};







