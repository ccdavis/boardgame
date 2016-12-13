


#ifndef parser_classes
#define parser_classes

#include<memory>
class Token;
class Range;

#include "scriptscan.h"
#include<stdio.h>
#include<string>


class ScriptParser
{
public:

    std::shared_ptr<Token> lookahead;
    ScriptScanner s;

    std::shared_ptr<Token>  last;
    char currentfile[100];
    FILE *f;
    ScriptParser(const std::string &file_name);
    ScriptParser();
    void stop();

    ~ScriptParser();

    void match(int c);
    void skip();
    int nextToken();
    const std::string nextTokenAsString();
    long nextTokenAsInteger();
    Range nextTokenAsRange();

    void error(int expect,int found);
    void error(const char * errstr);
    void error(const std::string &errstr);
};



#endif



