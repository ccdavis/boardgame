


#ifndef parser_classes
#define parser_classes
#include "token.h"
#include "scanner.h"
#include<memory>


class Token;
class Range;


#include<string>


class ScriptParser {
private:

    std::shared_ptr<Token> lookahead;
    ScriptScanner s;

    std::shared_ptr<Token>  last;

    std::string currentfile;

public:

    ScriptParser(const std::string &file_name);
    ScriptParser() {}
    void stop();


    void match(token_t c);
    void skip();
    token_t nextToken();

// Get values out of tokens
    std::string nextTokenAsString();
    std::string lastTokenAsString();
    int64_t nextTokenAsInteger();
    int64_t lastTokenAsInteger();
    Range nextTokenAsRange();
    Range lastTokenAsRange();
    double nextTokenAsFloat();
    double lastTokenAsFloat();
    bool nextTokenAsBool();
    bool lastTokenAsBool();
    token_t nextTokenAsOpcode();
    token_t lastTokenAsOpcode();
    void error(token_t expect, token_t found);
    void error(const std::string &errstr);
};

#endif
