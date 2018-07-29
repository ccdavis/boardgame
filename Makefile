

CXX=c++
CXXFLAGS=-I. -std=c++11 -pthread -Og -g --static
LINKFLAGS=-I . -std=c++11 -pthread -Og -g 

BUILD=./build

scanner: 
	$(CXX) $(CXXFLAGS) ./parsing/scanner.cpp -c -o  $(BUILD)/scanner.o

	
token: 
	$(CXX) $(CXXFLAGS) parsing/token.cpp -c -o $(BUILD)/token.o

parser: 
	$(CXX) $(CXXFLAGS) ./parsing/parser.cpp -c -o $(BUILD)/parser.o
	
game_parser: 
	$(CXX) $(CXXFLAGS) ./parsing/game_parser.cpp -c -o  $(BUILD)/game_parser.o

systems:
	$(CXX) $(CXXFLAGS) ./game/systems.cpp -c -o $(BUILD)/systems.o
	
components:
	$(CXX) $(CXXFLAGS) ./game/components.cpp -c -o $(BUILD)/components.o

tests: components game_parser systems token scanner parser
	$(CXX) $(CXXFLAGS)  tester.cpp -o tests $(BUILD)/systems.o $(BUILD)/components.o $(BUILD)/game_parser.o \
	$(BUILD)/token.o $(BUILD)/scanner.o $(BUILD)/parser.o
	
game:  game_parser systems components
	c++ -std=c++1y -o ./build/boardgame $(BUILD)/*.o
	
clean:
		rm -f ./build/*.o
		rm tests
		rm ./parsing/*.o
		