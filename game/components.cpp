

#include "components.hpp"
#include "parsing/game_parser.h"


/*
Maybe we'll want to move this logic for building a game object elsewhere later.

*/

using namespace std;


terrain_t  to_terrain_t(std::string type){
	if (type == "water"){
		return terrain_t::water;
	}else if (type =="land"){
		return terrain_t::land;
	}else if (type == "both"){
		return terrain_t::both;
	}else{
		throw std::domain_error("Unknown terrain type: " + type);
	}
}



Game::Game(const  GameState & loaded_game){
	
	map<string,Player*> players_by_name;
	map<string,Territory*> territory_by_name;
		
	for(auto p:loaded_game.players){
		Player player;
		player.name = p;
		players.push_back(player);	
		players_by_name[p] = & players.back();
	}

		// Search out ownership of territory
		// and assign basic attributes.	
	for(auto location_attr:loaded_game.territories){
			
		auto name =  location_attr.first;
		auto attributes = location_attr.second;
		auto owner = attributes["owner"];					
		
		Territory territory;		
		territory.name = attributes["name"];
		territory.terrain = to_terrain_t(attributes["type"]);
		territory.production = stoi(attributes["production"]);
		
		if (players_by_name.count(owner)==0){
			cerr << "Could not find a player named " << owner << " to assign to territory: " << territory.name << endl;
		}
						
		
		territory.owner =  players_by_name.at(	owner);
		
			board.push_back(territory);		
		territory_by_name[territory.name] = &board.back();
	}
	
	// Now connect territories to each other	
	for (Territory &t:board){	
		auto name = t.name;
		
		if (loaded_game.game_map.count(name)==0){
			cerr << "Cannot find any connections for territory named " << name << endl;
		}
		
		auto connections = loaded_game.game_map.at(name);
		
		for (auto connected_to:connections){
			if (territory_by_name.count(connected_to) == 0){
				cerr << "Cannot connect " << t.name << " to " << connected_to << " because no territory of exactly this name exists on the board." << endl;
				
			}
			
			t.connected_to.push_back(territory_by_name.at(connected_to));
		}		
	}
		
}
	
	
	
	
	
	
				


	
	
	


