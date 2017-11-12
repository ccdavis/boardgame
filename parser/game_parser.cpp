#include "game_parser.h"

using namespace std;

GameParser::GameParser(const string & game_script_file):
	ScriptParser(game_script_file){
		game = make_shared<GameState>();
	}


game_state_ptr GameParser::load(){
	players();
	turn();
	territories();
	game_map();
	units();
	containers();
	placement();
	return game;
}

void GameParser::players(){
	match(token_t::PLAYERS);
	while (token_t::IDENTIFIER == nextToken()){
		match(token_t::IDENTIFIER);
		string name = lastTokenAsString();
		game->players.push_back(name);
		if (nextToken() == token_t::COMMA)
			match(token_t::COMMA);
	}

	match(token_t::SEMICOLON);
}

void GameParser::turn(){
	match(token_t::TURN);
	match(token_t::INTEGER);
	game->turn = lastTokenAsInteger();
	match(token_t::SEMICOLON);
}

void GameParser::territories(){
	match(token_t::TERRITORIES);
	do{
		// The name of the territory is some words separated by spaces. I didn't want
		// to force quotes to be around the name, so we just read the parts  until
		// finding the  : symbol. Plus this way we can fix whitespace
		// errors later in the file; we force one space between name parts.
		vector<string> name_parts;
		while (nextToken() != token_t::COLON){
			match(token_t::IDENTIFIER);
			string part = lastTokenAsString();
			name_parts.push_back(part);
		}
		string territory_name = "";
		for(int p=0;p<name_parts.size();p++){
			territory_name += name_parts.at(p);
			if (p < name_parts.size() -1){
				territory_name += " ";
			}
		}

		match(token_t::COLON);
		match(token_t::IDENTIFIER);
		string territory_type = lastTokenAsString();
		match(token_t::COMMA);
		match(token_t::IDENTIFIER);
		string owner = lastTokenAsString();
		match(token_t::COMMA);
		match(token_t::INTEGER);
		int production_points = lastTokenAsInteger();
		match(token_t::SEMICOLON);
		map<string,string> this_territory = {
			{"name",territory_name},
			{"type",  territory_type},
			{"owner", owner},
			{"production", to_string(production_points)}
		};

		if (game->territories.count(territory_name) > 0){
			error("Duplicate territory name: " + territory_name);
		}else{
			game->territories[territory_name] = this_territory;
		}

	}while(nextToken() != token_t::MAP);
}

void GameParser::game_map(){
}

void GameParser::units(){
}

void GameParser::containers(){
}

void GameParser::placement(){
}
