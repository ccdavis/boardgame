

#include "components.hpp"
#include "parser/game_parser.h"


/*
Maybe we'll want to move thislogic elsewhere later.

*/

using namespace std;

Game::Game(const  GameState & loaded_game){
	
	map<string,player_id> player_id_by_name;
		
	for(player_id p=0;p<loaded_game.players.size()-1;p++){			
		Player player;
		player.name = loaded_game.players[p];
		player_id_by_name[player.name] = p;
		players[p] = player;
	}

		// Search out ownership of territory
		int t_id = 0;
		for(auto location_attr:loaded_game.territories){
			
			auto name =  location_attr.first;
			auto attributes = location_attr.second;
			auto owner = attributes["owner"];					
			
			
			Territory territory;
			player_id id = player_id_by_name[owner];
			territory.owner = &players[id];
			board[t_id] = territory;
			t_id++;
			
		}
	
				
	}

	
	
	


