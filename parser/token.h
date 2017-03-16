

#pragma once

#include <map>
#include <string>

enum class token_t{
	UNKNOWN,
	END_OF_FILE,
	NEWLINE,
	COMMA,
	LEFT_BRACKET,
	RIGHT_BRACKET,
	LEFT_PAREN,
	RIGHT_PAREN,
	LEFT_BRACE,
	RIGHT_BRACE,
	INTEGER,
	FLOAT,
	STRING,
	RANGE,
	IDENTIFIER,
	SEMICOLON,
	COLON,
	PLUS,
	MINUS,
	MULTIPLY,
	DIVIDE,
	LESSTHAN,
	GREATERTHAN,
	EQUAL,
	ASSIGN,
	AND,
	OR,
	NOT,
	IF,
	THEN,
	ELSE,
	WHILE,
	RETURN,
	FUNC,
	LIST,
	RECORD,
	READ,
	WRITE,
	MAP,
	FILTER,

	PLAYERS,
	TURN,
	TERRITORIES,
	UNITS,
	CONTAINERS,
	PLACEMENT

};

// When the token is a word
const std::map<std::string,token_t> reserved_words={
 {"and", token_t::AND},
 {"or", token_t::OR},
 {"not", token_t::NOT},
 {"range",token_t::RANGE},
 {"string",token_t::STRING},
 {"float",token_t::FLOAT},
 {"integer",token_t::INTEGER},
 {"if",token_t::IF},
 {"then",token_t::THEN},
 {"else",token_t::ELSE},
 {"while",token_t::WHILE},
 {"return",token_t::RETURN},
 {"func",token_t::FUNC},
 {"list",token_t::LIST},
 {"record",token_t::RECORD},
 {"read",token_t::READ},
 {"write",token_t::WRITE},
 {"map",token_t::MAP},
 {"filter",token_t::FILTER},

 // Game tokens
 {"players",token_t::PLAYERS},
 {"turn",token_t::TURN},
 {"units",token_t::UNITS},
 {"containers",token_t::CONTAINERS},
 {"territories",token_t::TERRITORIES},
 {"placement",token_t::PLACEMENT}
};

const std::map <token_t,std::string> token_name = {
	{token_t::UNKNOWN,"unknown"},
	{token_t::END_OF_FILE,"END OF FILE **"},
	{token_t::NEWLINE,"newline"},
	{token_t::COMMA,"comma"},
	{token_t::LEFT_BRACKET,"left bracket"},
	{token_t::RIGHT_BRACKET,"right bracket"},
	{token_t::LEFT_PAREN,"left parentheses"},
	{token_t::RIGHT_PAREN,"right parentheses"},
	{token_t::LEFT_BRACE,"left brace"},
	{token_t::RIGHT_BRACE,"right brace"},
	{token_t::INTEGER,"integer"},
	{token_t::FLOAT,"float"},
	{token_t::STRING,"string"},
	{token_t::RANGE,"range"},
	{token_t::IDENTIFIER,"identifier"},
	{token_t::SEMICOLON,"semicolon"},
	{token_t::COLON,"colon"},
	{token_t::PLUS,"plus sign"},
	{token_t::MINUS,"minus sign"},
	{token_t::MULTIPLY,"multiply (*)"},
	{token_t::DIVIDE,"divide (/)"},
	{token_t::LESSTHAN,"<"},
	{token_t::GREATERTHAN,">"},
	{token_t::EQUAL,"=="},
	{token_t::ASSIGN,"assignment"},
	{token_t::AND,"and"},
	{token_t::OR,"or"},
	{token_t::NOT, "not"},
	{token_t::IF,"if"},
	{token_t::THEN,"then"},
	{token_t::ELSE,"else"},
	{token_t::WHILE,"while"},
	{token_t::FUNC,"function"},
	{token_t::RETURN,"return"},
	{token_t::LIST,"list"},
	{token_t::RECORD,"record"},
	{token_t::READ,"read"},
	{token_t::WRITE,"write"},
	{token_t::MAP,"map"},
	{token_t::FILTER,"filter"},
	{token_t::PLAYERS,"players"},
	{token_t::TURN,"turn"},
	{token_t::TERRITORIES,"territories"},
	{token_t::UNITS,"units"},
	{token_t::CONTAINERS,"containers"},
	{token_t::PLACEMENT,"placement"}
};



const int max_label_len = 256;

// The special range type, declared like lo..hi, for example 23..25
class Range {
	public:
    int64_t lo;
    int64_t hi;
};


union Attrib {
    int64_t integer_;
    Range range_;
    double float_;
    bool bool_;
    token_t opcode_;   // Specific operator such as <, > = (for relation_op, +,- for int_op, etc.
};


// The scanner will return new Token() using the appropriate constructor
class Token{
private:

    Attrib attr;
public:
    const std::string content;

    const token_t code;

    // anything which has a string label, including a string type itself
    Token(token_t code, const std::string & content):
    	code(code),content(content){}

    // a range of numbers written in code as 3..5 for example
    Token(token_t code, Range r):code(code){
		attr.range_.hi = r.hi;
		attr.range_.lo = r.lo;
	}

    Token(token_t code, double n):code(code){
		attr.float_ = n;
	}

	Token(token_t code, int64_t n):code(code){
		attr.integer_ = n;
	}

    // for reserved words needing no attribute
    Token(token_t code):code(code){}

    int64_t intValue(){ return attr.integer_;}
    std::string stringValue() { return content; }
    Range rangeValue() { return attr.range_; }
    double floatValue() { return attr.float_; }
    bool boolValue() { return attr.bool_; }
    token_t opcodeValue() { return attr.opcode_; }

    std::string print();
};



