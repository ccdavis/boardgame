
#include "systems.hpp"
#include "components.hpp"
#include "parsing/game_parser.h"

#include<iostream>

using namespace std;

void testing::game_loader(){
	
	// Parses the *.gdf format
	GameParser parser("aaa.gdf");
	
	// We could swap out the custom format that requires the
	// recursive descent parser with JSON, YML or something
	// else, while leaving the  game_state as-is. This way
	// the creation of the Game data structure and its members
	// is decoupled from the parser.
	auto game_state = parser.load();
	
	// The game constructor knows nothing of the original game
	// definition language, making future serialization and 
	// deserialization code easier to write.
	Game game(*game_state);
	
	// Grabs references to unique_ptr in the players
	// vector, does not transfer ownership, each item
	// is treated as a pointer with ->
	for(const auto &p:game.players){
		cout << "player: " << p->name << endl;
	}
	
	// You can also dereference on unique_ptr to get
	// away from the -> pointer syntax
	for (int t=0;t<game.board.size();t++){
		const Territory & ter =  *game.board[t];
		cout << "Territory " << ter.name << endl;
		
	}
		
	
	
	
	
	
}
