
#include"tests.hpp"
#include "components.hpp"
#include "systems.hpp"
#include "parsing/game_parser.h"

#include<iostream>
#include<assert.h>

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

    assert(game.players.size()>0);
    

    
    for (const Territory & t:game.board){
        /*
        cout << "Territory " << t.name << endl;
		cout << "\t" << t.owner.name << endl;
		cout << "\t\t ";
		for(const Territory  &adjacent:t.connected_to){			
			cout << adjacent.name << ", ";
		}
		cout << endl;
		*/
    }
}

void  change_ownership(){
	GameParser parser("aaa.gdf");
	auto game_state = parser.load();
	Game game(*game_state);
	
	
	Player & player0 =game.players[0];
	Player & player2 = game.players[2];
	
	
	Territory &  t1 = player0.territories[0];
	Territory &  t2 = player2.territories[0];
	
	Player & p3 = game.players[3];
	Player & p4 = game.players[4];
	
	Territory &  t3 = p3.territories[0];
	
	assert(t3.owner == &p3);
	
	assert(t3.owner->name != p4.name);
	cout << "t3.name == " << t3.name << endl;
	
	game_board::change_ownership(t3, p4);
	cout << "t3.name == " << t3.name << endl;
	assert(t3.owner->name ==  p4.name);
	
	p4.name = "fake";
	cout << "addresses p4: " << long(&p4) << " t3.owner: " << long(t3.owner) << endl;
	
	cout << "t3.owner->name == " << t3.owner->name << endl;
	assert(t3.owner->name == "fake");
	
	
	Territory & new_t3 = p3.territories[0];
	assert(&new_t3 != &t3);
	
	
	
	
}

};
