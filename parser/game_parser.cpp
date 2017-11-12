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
		match(token_t::IDENTIFIER);
		string territory_name = lastTokenAsString();
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

	}while(nextToken != token_t::MAP);
}

void GameParser::game_map(){
}

void GameParser::units(){
}

void GameParser::containers(){
}

void GameParser::placement(){
}
