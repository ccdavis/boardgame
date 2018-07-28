
	
	
#pragma once


#include<string>
#include<vector>
#include<unordered_map>
#include<map>
	
	
enum class terrain_t{
		land, water, both
};
	
	


struct Territory;

struct PieceTemplate{
	std::map<std::string, int> capacity;
	std::map<std::string, std::vector<std::string>> can_carry;
	std::map<std::string,int> cost;	
};



struct Piece{	
	std::unordered_map<int,Territory*> location;	
	std::unordered_map<int, int> movement;
	std::unordered_map<int, int> attack;
	std::unordered_map<int, int> defend;
	std::unordered_map<int, terrain_t> terrain;
	std::unordered_map<int, std::string> name;	
};



struct Player{
	std::string name;	
	bool npc;
	bool active;
	std::vector<Territory*> territories;	
	
	PieceTemplate piece_templates;
};


struct Territory{
	std::string name;
	Player * owner;
	std::vector<int> pieces;
	terrain_t terrain;
	int production;
	std::vector<Territory*> connected_to;
};



struct Game{		
	std::vector<Territory*> board;
	std::vector<Player*> players;		
	Piece pieces;
	PieceTemplate global_piece_templates;
};
	






