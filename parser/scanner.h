
#ifndef ScriptScanner_class
#define ScriptScanner_class
#include<memory>
#include <iostream>
#include <fstream>

class Token;


class ScriptScanner {
	private:
	std::ifstream infile;

	public:

	char lastchar;

    long line;

    bool whitespace(char c);
    bool digit(char c);
    bool letter(char c);

    void skipline();

    char nextChar();

    std::shared_ptr<Token> nextToken();

    void start(const std::string & filename);


    ScriptScanner() {
		lastchar = '\0';
    }


    ScriptScanner(const std::string & filename){
		lastchar = '\0';
        start(filename);
    }
};

#endif

