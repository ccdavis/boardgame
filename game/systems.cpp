
#include "systems.hpp"
#include "components.hpp"
#include "parsing/game_parser.h"

#include<iostream>

using namespace std;

void testing::game_loader(){
	GameParser parser("aaa.gdf");
	auto game_state = parser.load();
	
	Game game(*game_state);
	
	for(auto &p:game.players){
		cout << "player: " << p.name << endl;

	}
	
	for (int t=0;t<game.board.size();t++){
		Territory & ter =  game.board[t];
		cout << "Territory " << ter.name << endl;
		
	}
		
	
	
	
	
	
}
