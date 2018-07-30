#include "game_parser.h"

using namespace std;

int main() {

    GameParser game("../aaa.gdf");
    auto game_state = game.load();

    return 0;
}

