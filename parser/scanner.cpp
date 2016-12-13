
#include "scriptscan.h"
#include "tconst.h"
#include "token.h"
#include "classic/common.h"

#include <string.h>
int ScriptScanner::whitespace(char c) {
    char ws[4];
    ws[0]=(char) 32;
    ws[1]=(char) 13;
    ws[2]='\n';
    ws[3]='\t';

    for (unsigned int i=0; i<sizeof(ws); i++)
        if (c==ws[i]) return 1;
    return 0;
}

int ScriptScanner::digit(char c) {
    int r=0;
    char d[]= "0123456789";

    for (unsigned int i=0; i<sizeof(d)-1; i++) if (d[i]==c) r=1;
    return r;
}

void ScriptScanner::skipline() {
    while (lastchar[0]!='\n') nextChar();
}

char ScriptScanner::nextChar() {

    lastchar[0]=(char) fgetc(f);
    if (lastchar[0]=='\n') {
        line++;
    }
    return lastchar[0];

}

std::shared_ptr<Token> ScriptScanner::nextToken() {
    char cstr[max_token_length];
    strcpy(cstr,"");

    while (whitespace(lastchar[0])) nextChar();
    if (lastchar[0]==EOF) return make_shared<Token>(EOFtoken);


    // return number type tokens such as INTEGER, FLOAT, and RANGE
    if (digit(lastchar[0]) ||
            lastchar[0]=='-')
    {
        int flt=0;  // flag if number turns out to be floating point
        int rng=0;

        do
        {
            strcat(cstr,lastchar);
            nextChar();
        } while (digit(lastchar[0]));
        long r1 =atol(cstr);

        if (lastchar[0]=='.') {
            strcat(cstr,lastchar);
            flt=1;
            nextChar();
            if (lastchar[0]=='.') {
                rng=1;
                flt=0;
                nextChar();
            }

        }

        char cstr2[25];
        strcpy(cstr2,"");
        while (digit(lastchar[0])) {

            strcat(cstr,lastchar);
            strcat(cstr2,lastchar);
            nextChar();

        }
        long r2=atol(cstr2);

        // there was at least one digit so return the number token type
        if (flt)
            return make_shared<Token>(FLOAT,atof(cstr),cstr);

        if (rng) {
            Range r;
            r.lo=r1;
            r.hi=r2;
            return make_shared<Token>(RANGE,r);
        }


        return make_shared<Token>(INTEGER,atol(cstr),cstr);

    } // if number

    if (lastchar[0]=='"') {
        nextChar();
        while (lastchar[0]!='"') {
            strcat(cstr,lastchar);
            nextChar();
            if ((lastchar[0]=='\t') || (lastchar[0]=='\n'))
                cerr << "Scann error:  Unterminated string on line " << line << "\n";
        }
        nextChar();
        return make_shared<Token>(STRING,cstr);
    }


    // now let's look for reserved words
    if (letter(lastchar[0])) {
        strcat(cstr,lastchar);
        nextChar();

        while (letter(lastchar[0]) || digit(lastchar[0])) {
            strcat(cstr,lastchar);
            nextChar();
        }  // at the end of a reserved word or identifier token, now which is it?

        int thisCode=_unknown;

        for (int i=0; i<top_word; i++) if (strcasecmp(cstr,name[i])==0) thisCode=i;

        // Check for a labeled range  (like P25..28)
        if (lastchar[0]=='.') {
            strcat(cstr,lastchar);
            nextChar();
            if (lastchar[0]=='.') {
                strcat(cstr,lastchar);
                nextChar();
                if (digit(lastchar[0])) {
                    while (digit(lastchar[0])) {
                        strcat(cstr,lastchar);
                        nextChar();
                    }
                    // return a new "Labeled range" token
                    char savedlastchar=lastchar[0];

                    // get label
                    char lbl[25];
                    strcpy(lbl,"");
                    int i=0;
                    while (letter(cstr[i])) {
                        lastchar[0]=cstr[i];
                        strcat(lbl,lastchar);
                        i++;
                    }
                    // get first number
                    char r1str[25];
                    strcpy(r1str,"");
                    while (digit(cstr[i])) {
                        lastchar[0]=cstr[i];
                        strcat(r1str,lastchar);
                        i++;
                    }
                    long r1 = atol(r1str);

                    // move past the ".."
                    i++;
                    i++;

                    char r2str[25];
                    strcpy(r2str,"");
                    while (digit(cstr[i])) {
                        lastchar[0]=cstr[i];
                        strcat(r2str,lastchar);
                        i++;

                    }
                    long r2=atol(r2str);
                    Range r;
                    r.lo=r1;
                    r.hi=r2;
                    lastchar[0]=savedlastchar;

                    return make_shared<Token>(LABELED_RANGE,lbl,r.lo,r.hi);
                } // if
            }
        }


        if (thisCode==_unknown) return make_shared<Token>(IDENTIFIER,cstr);
        else return make_shared<Token>(thisCode);
    } // if letter

    // skip the rest of a line (for comments)

    char ch = lastchar[0];
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

int ScriptScanner::letter(char c) {
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


