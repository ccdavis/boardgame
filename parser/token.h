

#pragma once

#include <map>
#include <string>

enum class token_t{
};

const std::map <token_t,std::string> = {
	{token_t::end_of_file,"END OF FILE **"},
	{token_t::newline,"newline"},
	{token_t::comma,"comma"},
	{token_t::left_bracket,"left bracket"},
	{token_t::right_bracket,"right bracket"},
	{token_t::left_paren,"left parentheses"},
	{token_t::right_paren,"right parentheses"},
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
};

const static  char * name[topcode] = {

    "<INPUTCOLUMNS>",
    "<INPUTCOLUMS>",
    "FLAG",
    "<description>",
    "CODE",
    "NOCODE",
    "LABEL",
    "FREQ",
    "<begin>",
    "<end>",
    "<RAWOUTPUT>",
    "VARIABLES",
    "OUTPUT",
    "FORMATNAME",
    "NONTABULATED",
    "FORMATFILE",
    "FILENAME",
    "CONVERT",
    "DOCUMENTATION",
    "TESTEDIT",
    "TESTALLOCATION",
    "SERIALNUMBER",
    "ALLOCATED",
    "WHEN",
    "spss",
    "yaml",
    "stata"
    "PROJECT",
    "CONFIG",
    "<dataset>",
    "COLUMN DELIMITER", // TAB for now
    "NEW LINE",  // probably look for the CR/LF pair
    "End of File ***", // look for EOF char
    "Float",
    "Integer",
    "_ALL_TYPES_",
    "IDENTIFIER",
    "RANGE",
    "SPECIAL",
    "LABELED RANGE",
    "STRING",
    "COMMA",
    "SEMICOLON",
    "EQUAL",
    "LEFTPAREN",
    "RIGHTPAREN",
    "LEFTBRACKET",
    "RIGHTBRACKET",

    "LEFT ARROW",
    "RIGHT ARROW"
};



const int max_lexeme_len=250;
#include "classic/common.h"


#include <string.h>

// Tokens may contain different types of data

// The special range type, declared like lo..hi, for example 23..25
class Range {
public:
    long lo;
    long hi;
};

struct Labeled_Range {
    long hi;
    long lo;
    char lbl[max_lexeme_len];
};

union Attrib {
    Labeled_Range lrange_;
    int integer_;
    long long_;
    Range range_;
    char label_[max_lexeme_len];

    float float_;
    int opcode_;   // Specific operator such as <, > = (for relation_op, +,- for int_op, etc.
};


// The scanner will return new Token() using the appropriate constructor
class Token
{
public:
    int code;
    Attrib attr;
    char lexeme[max_lexeme_len]; // the original string

    // anything which has a string label, including a string type itself
    Token(int c, char * l);

    // anything which has a string label, including a string type itself
    Token(int c, const char * l);

    // a range of numbers written in code as 3..5 for example
    Token(int c, Range r);

    Token(int c, char * l, long low,long high);


    Token(int c, double n);

    Token(int c,double n,char * l);


    // for reserved words needing no attribute
    Token(int c);




    // Throw away the string memory if the token's a string, etc.
    ~Token();
    void print();

};



