#include "token.h"
#include "parser.h"


#include <errno.h>
#include <exception>
#include <stdexcept>
#include <stdio.h>
#include <iostream>
#include <sstream>
#include <cstring>

using namespace std;



ScriptParser::ScriptParser(const string &file_name){
		currentfile = file_name;
        s.start(currentfile);
        lookahead = s.nextToken();
        last=nullptr;
}

void ScriptParser::stop(){
}

void ScriptParser::match(token_t c) {
	cout << lookahead->print() << endl;
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

token_t ScriptParser::nextToken() {
    return lookahead->code;
}

int64_t ScriptParser::nextTokenAsInteger() {
    return lookahead->intValue();
}

int64_t ScriptParser::lastTokenAsInteger() {
    return last->intValue();
}

string ScriptParser::nextTokenAsString() {
    return lookahead->stringValue();
}

string ScriptParser::lastTokenAsString() {
    return last->stringValue();
}


double ScriptParser::nextTokenAsFloat(){
	return lookahead->floatValue();
}

double ScriptParser::lastTokenAsFloat(){
	return last->floatValue();
}


Range ScriptParser::nextTokenAsRange() {
    return  lookahead->rangeValue();
}

Range ScriptParser::lastTokenAsRange() {
    return  last->rangeValue();
}


void ScriptParser::error(token_t expect, token_t found){
    cerr << ": PARSE ERROR:  Expecting " << token_name.at(expect) << " but found " << token_name.at(found) << endl;
    cerr << "On line " <<  s.line << " in file " << currentfile << endl;
    exit(1);
}

void ScriptParser::error(const string &errstr){
    cerr << "Parse error: " << errstr << endl;
    cerr << "On line " <<  s.line << " in file " << currentfile << endl;
    exit(1);
}


