
#include "scriptscan.h"

#include "token.h"



bool ScriptScanner::whitespace(char c) {
    char ws[4];
    ws[0]=(char) 32;
    ws[1]=(char) 13;
    ws[2]='\n';
    ws[3]='\t';

    for (unsigned int i=0; i<sizeof(ws); i++)
        if (c==ws[i]) return true;
    return false;
}

bool ScriptScanner::digit(char c) {
    int r=false;
    char d[]= "0123456789";

    for (unsigned int i=0; i<sizeof(d)-1; i++) if (d[i]==c) r=true;
    return r;
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

std::shared_ptr<Token> ScriptScanner::nextToken() {
	string cstr="";

    while (whitespace(lastchar)) nextChar();
    if (lastchar==EOF) return make_shared<Token>(EOFtoken);


    // return number type tokens such as INTEGER, FLOAT, and RANGE
    if (digit(lastchar) ||
            lastchar=='-')
    {
        int flt=0;  // flag if number turns out to be floating point
        int rng=0;

        do
        {
            cstr.push_back(lastchar);
            nextChar();
        } while (digit(lastchar));
        long r1 =stol(cstr);

        if (lastchar=='.') {
            cstr.push_back(lastchar);
            flt=1;
            nextChar();
            if (lastchar=='.') {
                rng=1;
                flt=0;
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
            return make_shared<Token>(FLOAT,stof(cstr),cstr);

        if (rng) {
            Range r;
            r.lo=r1;
            r.hi=r2;
            return make_shared<Token>(RANGE,r);
        }


        return make_shared<Token>(INTEGER,stol(cstr),cstr);

    } // if number

    if (lastchar=='"') {
        nextChar();
        while (lastchar!='"') {
            cstr.push_back(lastchar);
            nextChar();
            if ((lastchar=='\t') || (lastchar[0]=='\n'))
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

        token_t thisCode=token_t::_unknown;


        //  search all the token names for matches
        if (symbols.count(downcase(cstr)) > 0)
        	thisCode = symbols.at(downcase(cstr));



        if (thisCode==_unknown) return make_shared<Token>(IDENTIFIER,cstr);
        else return make_shared<Token>(thisCode);
    } // if letter

    // skip the rest of a line (for comments)

    char ch = lastchar;
    nextChar();

    switch (ch) {
    case ',':
        return make_shared<Token>(COMMA);
        break;
    case '(':
        return make_shared<Token>(LEFTPAREN);
        break;
    case ')':
        return make_shared<Token>(RIGHTPAREN);
        break;
    case '[':
        return make_shared<Token>(LEFTBRACKET);
        break;
    case ']':
        return make_shared<Token>(RIGHTBRACKET);
        break;
    case ';':
        return make_shared<Token>(SEMICOLON);
        break;
    case '=':
        return make_shared<Token>(EQUAL);
        break;
    case '<':
        if (lastchar[0]=='-') {
            nextChar();
            return make_shared<Token>(LEFTARROW);
        }
        break;
    case '-':
        if (lastchar[0]=='>') {
            nextChar();
            return make_shared<Token>(RIGHTARROW);
        }
        break;

    case '*':
        return make_shared<Token>(SPECIAL);
    case '#': {
        skipline();
        return nextToken();
    }
    break;

        /*case '<':if (lastchar[0]=='=')   {
        	nextChar();

        		return make_shared<Token>(_rel_op, le_);
        	}else return make_shared<Token>(_rel_op,lt_);
        	break;


        case '>':
        	 if (lastchar[0]=='='){
        		 nextChar();
        	return make_shared<Token>(_rel_op,ge_);
        	}else return make_shared<Token>(_rel_op,gt_);
        	break;
        */
    } // and you can have many more like (,),[,], :=, == which should be here

    // if we reached here,the character wasn't part of any token so report that
    // just to stdout for now
    cerr << "Unhandled  character " << ch << " " << (int) ch << "-- skipping" << "\n";


    return nextToken(); // recurse and grab the next character
    // I guess this will die if there are too many bad characters in a row  and the
    // stack overflows

}

bool ScriptScanner::letter(char c) {
    int r=0;
    char l[]="+:$abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ_-./";
    for (unsigned int i=0; i<sizeof(l); i++) if (l[i]==c) r=1;
    return r;
}

void ScriptScanner::start(FILE * infile) {
    f = infile;
    nextChar();
    lastchar[1]='\0';
}


