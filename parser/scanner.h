
#ifndef ScriptScanner_class
#define ScriptScanner_class
#include<memory>

class Token;
#include <stdio.h>
#include <stdlib.h>
#include<string.h>
class ScriptScanner {
private:
    FILE *f;  // open text file
public:
    char lastchar[2];
    long line;


    int whitespace(char c);

    int digit(char c);
    void skipline();

    char nextChar();

    std::shared_ptr<Token> nextToken();

    int letter(char c);
    void start(FILE * infile);
    ScriptScanner() {

        lastchar[0]='\0';
        lastchar[1]='\0';

    }

    ScriptScanner(FILE *infile) {

        lastchar[0]='\0';
        lastchar[1]='\0';

        start(infile);
    }
};

#endif

