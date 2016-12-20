

#pragma once

#include <map>
#include <string>

enum class token_t{
	END_OF_FILE,
	NEWLINE,
	COMMA,
	LEFT_BRACKET,
	RIGHT_BRACKET,
	LEFT_PAREN,
	RIGHT_PAREN,
	INTEGER,
	FLOAT,
	STRING,
	RANGE,
	LABELED_RANGE,
	IDENTIFIER,
	SEMICOLON,
	COLON,
	PLUS,
	MINUS,
	MULTIPLY,
	DIVIDE
	LESSTHAN,
	GREATERTHAN,
	EQUAL,
	AND,
	OR,
	NOT
};

const std::map <token_t,std::string> token_name = {
	{token_t::END_OF_FILE,"END OF FILE **"},
	{token_t::NEWLINE,"newline"},
	{token_t::COMMA,"comma"},
	{token_t::LEFT_BRACKET,"left bracket"},
	{token_t::RIGHT_BRACKET,"right bracket"},
	{token_t::LEFT_PAREN,"left parentheses"},
	{token_t::RIGHT_PAREN,"right parentheses"},
	{token_t::INTEGER,"integer"},
	{token_t::FLOAT,"float"},
	{token_t::STRING,"string"},
	{token_t::RANGE,{"range"},
	{token_t::LABELED_RANGE,"labeled_range"},
	{token_t::IDENTIFIER,"identifier"},
	{token_t::SEMICOLON,"semicolon"},
	{token_t::COLON,"colon"},
	{token_t::PLUS,"plus sign"},
	{token_t::MINUS,"minus sign"},
	{token_t::MULTIPLY,"multiply (*)"},
	{token_t::DIVIDE,"divide (/)"}
	{token_t::LESSTHAN,"<"},
	{token_t::GREATERTHAN,">"},
	{token_t::EQUAL,"=="},
	{token_t::AND,"and"},
	{token_t::OR,"or"},
	{token_t::NOT, "not"}
};



const int max_label_len = 256;

// The special range type, declared like lo..hi, for example 23..25
class Range {
	public:
    long lo;
    long hi;
};


union Attrib {
    int64_t integer_;
    Range range_;
    double float_;
    bool bool_;
    token_t opcode_;   // Specific operator such as <, > = (for relation_op, +,- for int_op, etc.
};


// The scanner will return new Token() using the appropriate constructor
class Token
{
	private:

    Attrib attr;
    std::string content;

    token_t code;

    public:

    // anything which has a string label, including a string type itself
    Token(token_t code, const std::string & content);

    // a range of numbers written in code as 3..5 for example
    Token(token_t code, Range r);
    Token(token_t code, double n);

    // for reserved words needing no attribute
    Token(token_t code);

    std::string print();
};



