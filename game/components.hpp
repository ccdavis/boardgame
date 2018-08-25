


#pragma once

#include<stdexcept>
#include <exception>
#include<string>
#include<vector>
#include<unordered_map>
#include<map>
#include<deque>
#include<memory>
#include<functional>

enum class terrain_t {
    land, water, both
};


terrain_t  to_terrain_t(std::string type);




class GameState;
struct Territory;
typedef int piece_id;



struct Piece {
	public:
    int16_t capacity;
    int16_t cost;
    int16_t attack;
    int16_t defend;
    int16_t movement;
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
    std::vector<std::reference_wrapper<Territory>> territories;
    std::map<std::string, Piece> piece_templates;
};
		
	
struct Territory {
	public:
    std::string name;
    Player & owner;
    std::vector<piece_id> pieces;
    terrain_t terrain;
    int production;
    std::vector<std::reference_wrapper<Territory>> connected_to;	
	Territory(Player & p):owner(p){}
};



struct Game {
	
	// Use deque because we'll be grabbing
	// references to elements. Unlike vectors,
	// dequeues don't change memory locations
	// of elements once allocated.
    std::deque<Territory> board;
    std::deque<Player> players;

    // piece_id is needed since pieces get added and removed
    // throughout the course of the game.
    std::unordered_map<piece_id,Piece> pieces;

    // One template per piece name	
	// We use the name of the piece type ("armor", "battleship" etc as IDs 
    std::map<std::string,Piece> global_piece_templates;

    Game(const GameState & loaded_game);
};







