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
}

void GameParser::turn(){
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
