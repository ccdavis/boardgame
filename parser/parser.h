


#ifndef parser_classes
#define parser_classes

#include "scanner.h"
#include<memory>


class Token;
class Range;


#include<string>


class ScriptParser{
public:

    std::shared_ptr<Token> lookahead;
    ScriptScanner s;

    std::shared_ptr<Token>  last;

    std::string currentfile;

    FILE *f;
    ScriptParser(const std::string &file_name);
    ScriptParser(){}
    void stop();


    void match(token_t c);
    void skip();
    token_t nextToken();
    const std::string nextTokenAsString();
    long nextTokenAsInteger();
    Range nextTokenAsRange();

    void error(token_t expect, token_t found);
    void error(const std::string &errstr);
};

#endif
