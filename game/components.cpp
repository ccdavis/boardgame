

#include "components.hpp"
#include "parsing/game_parser.h"

#include<memory>
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
		unique_ptr<Player> player = make_unique<Player>();		
		player->name = p;
		
		players_by_name[p] = player.get();
		players.push_back(move(player));			
	}

		// Search out ownership of territory
		// and assign basic attributes.	
	for(auto location_attr:loaded_game.territories){
			
		auto name =  location_attr.first;
		auto attributes = location_attr.second;
		auto owner = attributes["owner"];					
		
		auto territory = make_unique<Territory>();
		territory->name = attributes["name"];
		territory->terrain = to_terrain_t(attributes["type"]);
		territory->production = stoi(attributes["production"]);
		
		if (players_by_name.count(owner)==0){
			cerr << "Could not find a player named " << owner << " to assign to territory: " << territory->name << endl;
		}
						
		
		territory->owner =  players_by_name.at(	owner);
		territory_by_name[territory->name] = territory.get();
		cout << "Added " << territory->name << endl;
		board.push_back(move(territory));		
			
	}
	
	// Now connect territories to each other	
	for (const auto  &t:board){	
		auto name = t->name;
		
		if (loaded_game.game_map.count(name)==0){
			cerr << "Cannot find any connections for territory named " << name << endl;
		}
		
		auto connections = loaded_game.game_map.at(name);
		
		for (auto connected_to:connections){
			if (territory_by_name.count(connected_to) == 0){
				cerr << "Cannot connect " << t->name << " to " << connected_to << " because no territory of exactly this name exists on the board." << endl;
				
			}
			
			t->connected_to.push_back(territory_by_name.at(connected_to));
		}		
	}
	
	
	// now set up the piece templates for use in instantiating pieces for each player
	for(auto name_unit:loaded_game.units){
		auto piece = make_unique<Piece>();
		piece->name = name_unit.first;
		piece->capacity = 0; // default, may be reset by containers section
		auto attributes = name_unit.second;
		piece->cost = attributes.at("cost");
		piece->attack = attributes.at("attack");
		piece->defend = attributes.at("defend");
		piece->movement = attributes.at("movement");
		if (attributes.count("water")>0){
			piece->terrain = terrain_t::water;
		}else if (attributes.count("land")>0){
			piece->terrain = terrain_t::land;
		}else if (attributes.count("both") > 0){
			piece->terrain = terrain_t::both;
		}else if(attributes.count("air") >0){
			piece->terrain = terrain_t::both;
		}else{
			cerr << "ERROR: No known type of terrain found for piece named " << piece->name << endl;			
			exit(1);
		}
		
		if (loaded_game.containers.count(piece->name)>0){
			auto container = loaded_game.containers.at(piece->name);
			for(auto carries:container){
				string unit_name = carries.first;
				int capacity = carries.second;
				// capacityis redundant so assigning the last one is fine
				piece->capacity = capacity;
				piece->can_carry.push_back(unit_name);
						
			}
					
		} // any containers
		
		global_piece_templates[piece->name] = make_unique<Piece>(*piece);
	}
	
	// Now that we have a global_piece_templates set assign copies to each 
	// player, as their defaults may change according to the rules
	for(auto & player:players){
		for(const auto & p_template:global_piece_templates){
			string piece_name = p_template.first;			
			player->piece_templates[piece_name] = make_unique<Piece>(*p_template.second);
		}		
	}
	
	
	piece_id p_id = 0;
	
	for(auto placed:loaded_game.placement){
		auto territory_name = placed.first;
		auto pieces_placed = placed.second;
		
		if (territory_by_name.count(territory_name)==0){
			cerr << "ERROR: Cannot place a piece on territory named " << territory_name << " because no such territory was defined on the map." << endl;
		
		}
		
		Territory * territory = territory_by_name.at(territory_name);
				
		for (auto this_piece:pieces_placed){
			string piece_name = this_piece.first;
			int quantity = this_piece.second;
			
			// Now use the previously set up global_piece_templates to initialize this piece
			for (int q=0;q<quantity;q++){
				
				pieces[p_id] = make_unique<Piece>(*(global_piece_templates.at(piece_name)));
				territory->pieces.push_back(p_id);
							
				p_id++;
			}
			
		}
		
		
		
	}
	
	
	
	
	
	
	
		
}
	
	
	
	
	
	
				


	
	
	


