


#ifndef parser_classes
#define parser_classes

#include "scanner.h"
#include<memory>


class Token;
class Range;


#include<string>


class ScriptParser{
private:

    std::shared_ptr<Token> lookahead;
    ScriptScanner s;

    std::shared_ptr<Token>  last;

    std::string currentfile;

    public:

    ScriptParser(const std::string &file_name);
    ScriptParser(){}
    void stop();


    void match(token_t c);
    void skip();
    token_t nextToken();

// Get values out of tokens
    std::string nextTokenAsString();
    int64_t nextTokenAsInteger();
    Range nextTokenAsRange();
    double nextTokenAsFloat();
    bool nextTokenAsBool();
    token_t nextTokenAsOpcode();

    void error(token_t expect, token_t found);
    void error(const std::string &errstr);
};

#endif
