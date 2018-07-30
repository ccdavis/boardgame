
#include "systems.hpp"
#include "components.hpp"
#include<algorithm>	



using namespace std;


void game_board::change_ownership(Territory * territory, Player *from, Player*to){
	territory->owner = to;
	to->territories.push_back(territory);
		 	
  from->territories.erase(
		remove_if(begin(from->territories), end(from->territories),
                [&](Territory* t) { return t==territory; }),
          end(from->territories));
}
	
	
	
