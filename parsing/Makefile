
CXX=c++
CXXFLAGS=-I .. -std=c++11 -pthread -Ofast -g --static
LINKFLAGS=-I .. -std=c++11 -pthread -Ofast -g 


scanner:
	$(CXX) $(CXXFLAGS) scanner.cpp -c 

token:
	$(CXX) $(CXXFLAGS) token.cpp -c

parser: scanner token
	$(CXX) $(CXXFLAGS) parser.cpp -c 

game_parser: parser
	$(CXX) $(CXXFLAGS) game_parser.cpp -c
	
load_game_state: game_parser
	$(CXX) $(LINKFLAGS) load_game_state.cpp -o	load_game_state parser.o game_parser.o scanner.o token.o
	
	