

#include "token.h"

using namespace std;

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


