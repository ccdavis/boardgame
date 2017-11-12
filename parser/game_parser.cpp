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
		if (nextToken() == token_t::COMMA)
			match(token_t::COMMA);
		game->players.push_back(name);
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
}

void GameParser::game_map(){
}

void GameParser::units(){
}

void GameParser::containers(){
}

void GameParser::placement(){
}
