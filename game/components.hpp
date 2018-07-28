
	
	
#pragma once


#include<string>
#include<vector>
#include<unordered_map>
#include<map>
	
	
enum class terrain_t{
		land, water, both
};
	
	

class GameState;
struct Territory;
typedef int piece_id;
typedef int territory_id;
typedef int player_id;

struct Piece{
	int capacity;
	int cost;	
	int attack;
	int defend;
	int movement;		
	terrain_t terrain;
	std::string name;
	std::vector<std::string> can_carry;
};

struct Player{	
	std::string name;	
	bool npc;
	bool active;
	std::vector<Territory*> territories;	
	std::map<std::string, Piece> piece_templates;	
};


struct Territory{	
	std::string name;
	Player * owner;
	std::vector<piece_id> pieces;
	terrain_t terrain;
	int production;
	std::vector<Territory*> connected_to;
};



struct Game{		
	// Players and territories are unchanging
	std::unordered_map<territory_id, Territory> board;
	std::unordered_map<player_id, Player> players;		
	
	// piece_id is needed since pieces get added and removed
	// throughout the course of the game.
	std::unordered_map<piece_id,Piece> pieces;
	
	// One template per piece name
	std::map<std::string,Piece> global_piece_templates;
	
	Game(const GameState & loaded_game);
};
	






