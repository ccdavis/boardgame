

CXX=c++
CXXFLAGS=-I. -std=c++11 -pthread -Ofast -g --static
LINKFLAGS=-I . -std=c++11 -pthread -Ofast -g 

BUILD=./build

scanner: 
	$(CXX) $(CXXFLAGS) ./parser/scanner.cpp -c -o  $(BUILD)/scanner.o

token: 
	$(CXX) $(CXXFLAGS) parser/token.cpp -c -o $(BUILD)/token.o

parser: 
	$(CXX) $(CXXFLAGS) ./parser/parser.cpp -c  -o $(BUILD)/parser.o

game_parser: parser	 scanner token
	$(CXX) $(CXXFLAGS) ./parser/game_parser.cpp -c -o  $(BUILD)/game_parser.o

systems:
	c++ -std=c++1y -I. ./game/systems.cpp -c $(BUILD)/systems.o
	
components:
	g++ -std=c++11 -I. ./game/components.cpp -c -o $(BUILD)/components.o

game:  game_parser systems components
	c++ -std=c++1y -o ./build/boardgame $(BUILD)/*.o
	