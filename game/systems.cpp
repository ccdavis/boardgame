
#include "systems.hpp"
#include "components.hpp"
#include<algorithm>



using namespace std;



void game_board::change_ownership(Territory *territory, Player*to) {
	/*
	Player * from = territory->owner;
    territory->owner = to;
    to->territories.push_back(territory);

    from->territories.erase(
        remove_if(begin(from->territories), end(from->territories),
    [&](Territory* t) {
        return t==territory;
    }),
    end(from->territories));
	
	*/
}


// Same as above but with pass-by-reference
void game_board::change_ownership(Territory &territory, Player &to) {
	Player &  from = *territory.owner;	
    territory.owner = &to;
    to.territories.push_back(territory);

    from.territories.erase(
        remove_if(begin(from.territories), end(from.territories),
    [&](Territory t) {
        return t.name==territory.name;
    }),
    end(from.territories));
}

