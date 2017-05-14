#pragma once

#include <vector>
#include <map>
#include <string>
#include <set>

namespace persist{

// This class holds an intermediate representation of the game state which is suitable
// for persisting to JSON or whatever. It is responsible for checking that data added to the
// game state is consistent as much as possible. The parser can call the add_* methods and if a logical error  is detected
// an excdeption gets thrown which can be caught in the parser and a line number given back to the user to help correct
// the error.
class Load{
	private:

// Names of players, referred to in the game map and placement sections
	std::set<std::string> players;
	std::vector<std::string> player_order;

	int turn;
	std::string next_player;

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

	public:

	void addPlayer(std::string name);
	void  setTurn(int turn);
	void  setCurrentPlayer(std::string name);
	void addTerritory(std::string name, std::string type, std::string owner, int yield);
	void add_map_location(std::string name, std::vector<std::string> connections);

	// Attributes must be attack,defend,movement,cost. Others are possible.
	void add_unit_type(std::string name, std::string type, std::map<std::string,int> attributes);
	void add_container_type(std::string unit_name, int capacity, std::vector<std::string> can_contain);
	void place_unit(std::string territory_name, std::string unit_type, int number);



    };
};
