
#include "tconst.h"
#include "token.h"


Token::Token(int c, char * l)
{


    strncpy(attr.label_,l,max_lexeme_len);
    code=c;
    strncpy(lexeme,l,max_lexeme_len);


}

Token::Token(int c, const char * l)
{


    strncpy(attr.label_,l,max_lexeme_len);
    code=c;
    strncpy(lexeme,l,max_lexeme_len);


}

Token::Token(int c, Range r)
{
    attr.range_.lo=r.lo;
    attr.range_.hi=r.hi;
    code=c;
    strcpy(lexeme,"");

}


Token::Token(int c, char * l, long low,long high)
{
    code = c;

    strncpy(attr.lrange_.lbl,l,max_lexeme_len);

    attr.lrange_.lo=low;
    attr.lrange_.hi=high;
    strcpy(lexeme,"");

}


Token::Token(int c, double n)
{
    if (c==FLOAT) attr.float_=n;
    if (c==INTEGER) attr.long_= (long) n;
    code=c;
    strcpy(lexeme,"");

}

Token::Token(int c,double n,char * l)
{
    code=c;
    if (c==FLOAT) attr.float_=n;
    if (c==INTEGER) attr.long_= (long) n;
    strncpy(lexeme,l,max_lexeme_len);


}

Token::Token(int c)
{
    strcpy(attr.lrange_.lbl,"");


    attr.lrange_.lo=0;
    attr.lrange_.hi=0;
    code=c;
    strcpy(lexeme,"");

}


Token::~Token()
{



}

void Token::print()
{   // display this token
    cout << name[code];
    if (code==IDENTIFIER) cout << " " << attr.label_;
    if (code==LABELED_RANGE)
        cout << " " << attr.lrange_.lbl << " " << attr.lrange_.lo << ".." << attr.lrange_.hi;
    if (code==RANGE)
        cout << " " << attr.range_.lo << ".." << attr.range_.hi;
    if (code==STRING)
        cout << " " << attr.label_;
    cout << endl;




}


