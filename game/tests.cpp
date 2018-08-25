
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
		cout << "\t" << t.owner.name << endl;
		cout << "\t\t ";
		for(const Territory  &adjacent:t.connected_to){			
			cout << adjacent.name << ", ";
		}
		cout << endl;
    }
}

void  change_ownership(){
	GameParser parser("aaa.gdf");
	auto game_state = parser.load();
	Game game(*game_state);
	
	// Everything on the 'game' instance is on the stack; we can make_heap
	// pointers to the items in the vectors when necessary:
	Player & player0 =game.players[0];
	Player & player2 = game.players[2];
	
	//  Everything not on 'game' pointing back into
	// game members is a pointer:	
	Territory &  t1 = player0.territories[0];
	Territory &  t2 = player2.territories[0];
	
	
	// Most of the time we could use references into the various game
	// members (e.g. Player & p1 = game.players[1] ) but the ownership
	// change code requires exchanging pointers.
	
	/*
	cout << "t1 owned by " << t1->owner->name << " t2 owned by " << t2->owner->name << endl;	
	game_board::change_ownership(t1, player2);
	cout << "t1 owned by " << t1->owner->name << " t2 owned by " << t2->owner->name << endl;
	game_board::change_ownership(t2, &(game.players[0]));
	cout << "t1 owned by " << t1->owner->name << " t2 owned by " << t2->owner->name << endl;
	*/
	
	// The reference version of above, not sure if it's a better approach
	Player & p3 = game.players[3];
	Player & p4 = game.players[4];
	
	Territory &  t3 = p3.territories[0];
	
	cout << " t3 owned by " <<  t3.owner.name << endl;
	game_board::change_ownership(t3, p4);
	cout << " t3 owned by " <<  t3.owner.name << endl;	
}

};
