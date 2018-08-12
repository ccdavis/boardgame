

#pragma once
class Territory;
class Player;

namespace game_board {

	void change_ownership(Territory * territory, Player *to);

	void change_ownership(Territory & territory, Player & to);
};

