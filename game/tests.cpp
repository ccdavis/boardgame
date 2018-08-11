
#include"tests.hpp"
#include "components.hpp"
#include "systems.hpp"
#include "parsing/game_parser.h"

#include<iostream>

using namespace std;

namespace testing {
void game_loader() {

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

    
    for(const Player &p:game.players) {
        cout << "player: " << p.name << endl;
    }


    
    for (const Territory & t:game.board){
        
        cout << "Territory " << t.name << endl;
		cout << "\t" << t.owner->name << endl;
		cout << "\t\t ";
		for(Territory  *adjacent:t.connected_to){			
			cout << adjacent->name << ", ";
		}
		cout << endl;

    }


}
};
