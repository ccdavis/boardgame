#include "game_parser.h"
#include <iostream>

using namespace std;


int main() {

    GameParser game("../aaa.gdf");
    auto game_state = game.load();
    cout << game_state->as_json() << endl;

    return 0;
}

