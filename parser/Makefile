
CXX=c++
CXXFLAGS=-I .. -std=c++11 -pthread -Ofast -g --static

scanner:
	$(CXX) $(CXXFLAGS) scanner.cpp -c 

token:
	$(CXX) $(CXXFLAGS) token.cpp -o parse_game

parser: scanner token
	$(CXX) $(CXXFLAGS) parser.cpp -o parse_game scanner.o token.o

