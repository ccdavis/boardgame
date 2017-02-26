

#include "token.h"

using namespace std;

Token::Token(token_t code, const string & content){
    this->code = code;
    this->content = content;
}


Token::Token(token_t code, Range r){
    attr.range_.lo=r.lo;
    attr.range_.hi=r.hi;
    this->code=code;
}



Token::Token(token_t code, double n){
    if (code==token_t::FLOAT) attr.float_=n;
    if (code==token_t::INTEGER) attr.integer_= long(n);
    this->code = code;
}

Token::Token(token_t code){
	this->content = "";
    attr.range_.lo=0;
    attr.range_.hi=0;
    this->code = code;
}

string Token::print(){
	string name =  token_name.at(code); + ": ";
	string value = "";
	switch(code){
		case token_t::IDENTIFIER:value = content;break;
		case token_t::FLOAT:value = to_string(attr.float_);break;
		case token_t::INTEGER:value = to_string(attr.integer_);break;
		case token_t::STRING:value = "'" + content + "'";break;
		case token_t::RANGE:value = to_string(attr.range_.lo) + ".." + to_string(attr.range_.hi);break;
		default:{}
	};

	return name + value;
}


