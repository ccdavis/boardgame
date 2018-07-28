
#include "systems.hpp"
#include "components.hpp"
#include "parser/game_parser.h"

#include<iostream>

using namespace std;

void testing::game_loader(){
	GameParser parser("aaa.gdf");
	auto game_state = parser.load();
	
	Game game(*game_state);
	
	for(auto p:game.players){
		
		auto id = p.first;
		Player player = p.second;
		
		cout << "ID: " << id << " player name: " << player.name << endl;
	}
	
	
	
}
