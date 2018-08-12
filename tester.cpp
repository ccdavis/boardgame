

/* There's a testing namespace in the systems code
which should exercise every aspect of the components
and the loading / saving code as well as the systems themselves.
*/



#include "game/tests.hpp"


using namespace std;

int main(){
	
	testing::game_loader();
	testing::change_ownership();
	
	return 0;
	
	
	
}


