
#include "parser.h"
#include<string.h>
#include <errno.h>
#include "classic/common.h"
#include <exception>
#include <stdexcept>
#include <stdio.h>
#include<cstring>

#include "tconst.h"
#include "token.h"
#include "scriptscan.h"
#include "libs/Logging.h"





ScriptParser::~ScriptParser()
{




}


ScriptParser::ScriptParser()
{
    //


}

ScriptParser::ScriptParser(const string &file_name)
{

    f=fopen(file_name.c_str(),"r");

    if (f==NULL || file_name.length()==0) {
        if (file_name.length() > 0) {
            ostringstream msg;
            msg << "Problem opening allocation script file named " <<  file_name << endl;

            msg << "Problem opening configuration or allocation script file named " << file_name << "." << endl;
            msg << "The file does not exist. DCP expects the final argument on the command line after the dataset and project name to be nothing or a script file name." << endl;
            msg << "The error was " << errno << ": " << strerror(errno) << endl;
            Logging::log_and_throw_runtime(msg.str());

        }
    } else {



        strcpy(currentfile,file_name.c_str());
        s.start(f);
        lookahead = s.nextToken();
        last=NULL;
    }
}

void ScriptParser::stop()
{
    if (f!=NULL) fclose(f);
    f=NULL;


}

void ScriptParser::match(int c) {
    if (lookahead->code==c) {

        last = lookahead;

        lookahead=s.nextToken();
    } else error(c,lookahead->code);
}

// Skip ahead one token without matching
void ScriptParser::skip() {

    last = lookahead;
    lookahead=s.nextToken();
}

int ScriptParser::nextToken() {
    return lookahead->code;
}

long ScriptParser::nextTokenAsInteger() {
    return lookahead->attr.long_;
}

const string ScriptParser::nextTokenAsString() {
    return string(lookahead->attr.label_);
}

Range ScriptParser::nextTokenAsRange() {
    return  lookahead->attr.range_;
}


void ScriptParser::error(int expect,int found)
{

    cerr << ": PARSE ERROR:  Expecting " << name[expect] << " but found " << name[found] << endl;
    cerr << "On line " <<  s.line << " in file " << currentfile << endl;
    exit(1);
}

void ScriptParser::error(const char * errstr)
{

    cerr << "Parse error: " << errstr << endl;
    cerr << "On line " << s.line << " in file " << currentfile << endl;
    exit(1);
}

void ScriptParser::error(const string &errstr)
{
    cerr << "Parse error: " << errstr << endl;
    cerr << "On line " <<  s.line << " in file " << currentfile << endl;
    exit(1);
}


