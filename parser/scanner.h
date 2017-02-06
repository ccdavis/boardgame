
#ifndef ScriptScanner_class
#define ScriptScanner_class

#include<memory>
#include <iostream>
#include <fstream>
#include <set>

class Token;

namespace character{

	const std::set<char> whitespace = {
		(char) 32,
		(char) 13,
		\n,
		\t
	};

	 const std::string = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ_";

	 const std::string digits = "0123456789";

};

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

