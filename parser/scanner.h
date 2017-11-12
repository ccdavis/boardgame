
#ifndef ScriptScanner_class
#define ScriptScanner_class

#include<memory>
#include <algorithm>
#include <iostream>
#include <fstream>
#include <set>
#include <string>

class Token;

namespace character{

	const std::set<char> whitespace = {
		(char) 32,
		(char) 13,
		'\n',
		'\t'
	};

	 const std::string letters= "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ_";

	 const std::string digits = "0123456789";

};

class ScriptScanner {
	private:

	static inline std::string upcase(const std::string &s) {
	    std::string s2 = s;
	    std::transform(s2.begin(), s2.end(), s2.begin(),   (int(*)(int)) toupper);
	    return s2;
	}

	static inline std::string downcase(const std::string &s) {
	    std::string s2 = s;
	    std::transform(s2.begin(), s2.end(), s2.begin(),   (int(*)(int)) tolower);
	    return s2;
	}

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
		line=0;
		lastchar = '\0';
    }


    ScriptScanner(const std::string & filename){
		line=0;
		lastchar = '\0';
        start(filename);
    }
};

#endif

