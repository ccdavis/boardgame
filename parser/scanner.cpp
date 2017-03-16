



#include "token.h"
#include "scanner.h"
#include<stdio.h>


using namespace std;

bool ScriptScanner::whitespace(char c) {
	return character::whitespace.count(c) > 0;
}

bool ScriptScanner::digit(char c) {
	return character::digits.find(c) != string::npos;
}


bool ScriptScanner::letter(char c){
	return character::letters.find(c) != string::npos;
}


void ScriptScanner::skipline() {
    while (lastchar!='\n') nextChar();
}

char ScriptScanner::nextChar() {
	lastchar = char(infile.get());

    if (lastchar=='\n') {
        line++;
    }

    return lastchar;
}

shared_ptr<Token> ScriptScanner::nextToken() {
	string cstr="";

    while (whitespace(lastchar)) nextChar();
    if (lastchar==EOF) return make_shared<Token>(token_t::END_OF_FILE);


    // return number type tokens such as INTEGER, FLOAT, and RANGE
    if (digit(lastchar) ||
            lastchar=='-'){
        bool flt=false;  // flag if number turns out to be floating point
        bool rng = false;


        do{
            cstr.push_back(lastchar);
            nextChar();
        } while (digit(lastchar));
        long r1 =stol(cstr);

        if (lastchar=='.') {
            cstr.push_back(lastchar);
            flt=true;
            nextChar();
            if (lastchar=='.') {
                rng=true;
                flt=false;
                nextChar();
            }
        }

        string cstr2 = "";
        while (digit(lastchar)) {
            cstr.push_back(lastchar);
            cstr2.push_back(lastchar);
            nextChar();
        }
        long r2=stol(cstr2);

        // there was at least one digit so return the number token type
        if (flt)
            return make_shared<Token>(token_t::FLOAT,stof(cstr));

        if (rng) {
            Range r;
            r.lo=r1;
            r.hi=r2;
            return make_shared<Token>(token_t::RANGE,r);
        }

        return make_shared<Token>(token_t::INTEGER,int64_t(stoll(cstr)));
    } // if number

    if (lastchar=='"') {
        nextChar();
        while (lastchar!='"') {
            cstr.push_back(lastchar);
            nextChar();
            if ((lastchar=='\t') || (lastchar=='\n'))
                cerr << "Scann error:  Unterminated string on line " << line << "\n";
        }
        nextChar();
        return make_shared<Token>(token_t::STRING,cstr);
    }


    // now let's look for reserved words
    if (letter(lastchar)) {
        cstr.push_back(lastchar);
        nextChar();

        while (letter(lastchar) || digit(lastchar)) {
			cstr.push_back(lastchar);
            nextChar();
        }  // at the end of a reserved word or identifier token, now which is it?

        token_t thisCode=token_t::UNKNOWN;

        //  search all the token names for matches
        if (reserved_words.count(downcase(cstr)) > 0)
        	thisCode = reserved_words.at(downcase(cstr));

        if (thisCode==token_t::UNKNOWN)
        	return make_shared<Token>(token_t::IDENTIFIER,cstr);
        else
        	return make_shared<Token>(thisCode);
    } // if letter

    // skip the rest of a line (for comments)
    char ch = lastchar;
    nextChar();

    switch (ch) {
    case ',':
        return make_shared<Token>(token_t::COMMA);
        break;
    case '(':
        return make_shared<Token>(token_t::LEFT_PAREN);
        break;
    case ')':
        return make_shared<Token>(token_t::RIGHT_PAREN);
        break;
    case '[':
        return make_shared<Token>(token_t::LEFT_BRACKET);
        break;
    case ']':
        return make_shared<Token>(token_t::RIGHT_BRACKET);
        break;
    case ';':
        return make_shared<Token>(token_t::SEMICOLON);
        break;
    case ':':
    	if(lastchar == '='){
			nextChar();
			return make_shared<Token>(token_t::ASSIGN);
		}else{
			return make_shared<Token>(token_t::COLON);
		}
    	break;
    case '=':
        return make_shared<Token>(token_t::EQUAL);
        break;
   case '<':
    	return make_shared<Token>(token_t::LESSTHAN);
        break;
   case '>':
   		return make_shared<Token>(token_t::GREATERTHAN);
   		break;
   case '-':
   		return make_shared<Token>(token_t::MINUS);
        break;
    case '+':
    	return make_shared<Token>(token_t::PLUS);
    	break;
	case '/':
		return make_shared<Token>(token_t::DIVIDE);
		break;
	case '{':
		return make_shared<Token>(token_t::LEFT_BRACE);
		break;
	case '}':
		return make_shared<Token>(token_t::RIGHT_BRACE);
		break;
    case '*':
        return make_shared<Token>(token_t::MULTIPLY);
        break;
    case '#': {
        skipline();
        return nextToken();
    }
    break;

    default:{


    // if we reached here,the character wasn't part of any token so report that
    // just to stdout for now
    	cerr << "Unhandled  character " << ch << " " << (int) ch << "-- skipping" << "\n";
		}
	}


    return nextToken(); // recurse and grab the next character
    // I guess this will die if there are too many bad characters in a row  and the
    // stack overflows

}

void ScriptScanner::start(const std::string & filename) {
	infile.open(filename);
    nextChar();
}


