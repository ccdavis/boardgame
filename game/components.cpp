

#include "components.hpp"
#include "parsing/game_parser.h"

#include<memory>

using namespace std;


terrain_t  to_terrain_t(std::string type) {
    if (type == "water") {
        return terrain_t::water;
    } else if (type =="land") {
        return terrain_t::land;
    } else if (type == "both") {
        return terrain_t::both;
    } else {
        throw std::domain_error("Unknown terrain type: " + type);
    }
}


/*
Maybe we'll want to move this logic for building a game object elsewhere later.

*/

Game::Game(const  GameState & loaded_game) {

	// Temporary collections while setting up the game state
    map<string,Player*> players_by_name;
    map<string,Territory*> territory_by_name;

	// These "player" items are just  the names of the players, not Player classes.
    for(auto p:loaded_game.players) {
        Player player;
        player.name = p;        
        players.push_back(player);
		players_by_name[p]= &players.back();
	}
		
	
	for(int p=0;p<players.size();p++){
		//cout << "Player: " << players[p].name << endl;
		string  n = players[p].name;
	
		cout << " Storing player_by_name: " << players_by_name.at(n)->name << endl;
	}
	

    // Search out ownership of territory
    // and assign basic attributes.
    for(auto location_attr:loaded_game.territories) {

        auto name =  location_attr.first;
        auto attributes = location_attr.second;
        auto owner = attributes["owner"];
		Player & player=  *players_by_name.at(owner);
		Territory territory(player);
        
        territory.name = attributes["name"];
        territory.terrain = to_terrain_t(attributes["type"]);
        territory.production = stoi(attributes["production"]);

        if (players_by_name.count(owner)==0) {
            cerr << "Could not find a player named " << owner << " to assign to territory: " << territory.name << endl;
			exit(1);
        }

		
        
        board.push_back(territory);
		Territory & this_territory = board.back();
		territory_by_name[name] = &this_territory;
		territory.owner->territories.push_back(this_territory);
		
	}

/*	
	for(size_t spot=0;spot<board.size();spot++){		
		Territory * t_ptr = &board[spot];
		Territory & t_ref = board[spot];
		
		Player * owned_by = t_ptr->owner;
		
		//cout << "Adding territory " << t_ptr->name << " to player: " << owned_by->name << endl;
		
		territory_by_name[t_ptr->name] =  t_ptr;				
		owned_by->territories.push_back(t_ref);
    }
*/
    // Now connect territories to each other
    for (Territory &t:board) {
        auto name = t.name;

        if (loaded_game.game_map.count(name)==0) {
            cerr << "Cannot find any connections for territory named " << name << endl;
        }

        auto connections = loaded_game.game_map.at(name);

        for (auto connected_to:connections) {
            if (territory_by_name.count(connected_to) == 0) {
                cerr << "Cannot connect " << t.name << " to " << connected_to << " because no territory of exactly this name exists on the board." << endl;
            }

            t.connected_to.push_back(*territory_by_name.at(connected_to));
        }
    }


    // now set up the piece templates for use in instantiating pieces for each player
    for(auto name_unit:loaded_game.units) {
        Piece piece;
        piece.name = name_unit.first;
        piece.capacity = 0; // default, may be reset by containers section
        auto attributes = name_unit.second;
        piece.cost = attributes.at("cost");
        piece.attack = attributes.at("attack");
        piece.defend = attributes.at("defend");
        piece.movement = attributes.at("movement");
        if (attributes.count("water")>0) {
            piece.terrain = terrain_t::water;
        } else if (attributes.count("land")>0) {
            piece.terrain = terrain_t::land;
        } else if (attributes.count("both") > 0) {
            piece.terrain = terrain_t::both;
        } else if(attributes.count("air") >0) {
            piece.terrain = terrain_t::both;
        } else {
            cerr << "ERROR: No known type of terrain found for piece named " << piece.name << endl;
            exit(1);
        }

        if (loaded_game.containers.count(piece.name)>0) {
            auto container = loaded_game.containers.at(piece.name);
            for(auto carries:container) {
                string unit_name = carries.first;
                int capacity = carries.second;
                // capacity is redundant so assigning the last one is fine
                piece.capacity = capacity;
                piece.can_carry.push_back(unit_name);
            }
        } // any containers

        global_piece_templates[piece.name] = piece;
    }

    // Now that we have a global_piece_templates set assign copies to each
    // player, as their defaults may change according to the rules
    for(Player & player:players) {
        for(const auto & p_template:global_piece_templates) {
            string piece_name = p_template.first;
			Piece piece = p_template.second;
            player.piece_templates[piece_name] = piece;
        }
    }


    piece_id p_id = 0;

    for(auto placed:loaded_game.placement) {
        auto territory_name = placed.first;
        auto pieces_placed = placed.second;

        if (territory_by_name.count(territory_name)==0) {
            cerr << "ERROR: Cannot place a piece on territory named " << territory_name << " because no such territory was defined on the map." << endl;
        }

        Territory & territory = *territory_by_name.at(territory_name);

        for (auto this_piece:pieces_placed) {
            string piece_name = this_piece.first;
            int quantity = this_piece.second;

            // Now use the previously set up global_piece_templates to initialize this piece
            for (int q=0; q<quantity; q++) {							
                pieces[p_id] = global_piece_templates.at(piece_name);
                territory.pieces.push_back(p_id);
                p_id++;
            }

        }

    }


}


