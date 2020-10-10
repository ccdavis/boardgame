# boardgame

This is a little project I use to explore various programming styles and concepts.

The latest work explored using a ECS (Entity Component System) design.

I want to explore using C++bindings in Python. It would be nice to be able to script the AI for the game in Python and change some aspects of the rules without rebuilding.

Next steps:
1. Use "pybind11" to produce a module importable by Python that exports the GameStorate struct type.
2. Make bindings for the Game and Piece  classes in addition to the basic GameStorage struct type, so that basic game types don't need to be redefined in Python.
3. Make an executible that embeds the Python interpreter and uses the bindings from 1,2 so that we have a program that loads games scripted in Python.
4. Make a separate GDF to JSON converter so the game definition could be read into Ruby or Crystal or whatever language and used as the basis for an implementation of the game, without needing to rewrite the parser from scratch.

-------------------------------------------------


The "aaa.gdf" file has a complete description for the starting state of a strategy board game similar to "Axis and Allies."

The "parser" directory has a complete modern C++ parser for this file along with a stand-alone test program and Makefile.

The format of "aaa.gdf" came about as a way to represent a game state that's machine readable but also very easy to edit with out introducing mistakes in the game description file. At the time I developed it YML and ToML didn't exist; I think I'd use ToML or maybe JSON if I were starting over.

The parser is implemented as a simple recursive descent parser. It's nice as example code showing how to build sucha parser. One advantage it has over a generic YML / JSON parser is that it can very accurately discover mistakes in the data it's parsing and direct you to the exact line with fairly helpful explainations of what's wrong. While not exactly an equivalent , you might think of the parser as a schema to validate the game definition, where the schema is executible code. While changing the parser is harder than updating a schema the recursive descent parser style is relatively easy to understand and modify.


Years ago I wrote  this game with Borland Pascal and intended to build the front-end with Delphi but never did. It ran in text mode mostly as a way to test the game. I got about 90% done and set it aside.


The game setup found in "aaa.gdf" was all in a BP unit  declared with Pascal types. This is actually surprisingly (well, perhaps surprising if you don't know Pascal) easy to read, sort of like Rails' practice of using Ruby for configuration.

Later when I decided to port the game to Java I extracted the game setup from the BP unit and wrote it out in a generically  parsible format.  Using the parser really helped to detect my many mistakes. I never got much beyond the parser with the Java version.

This C++ version pretty much  mirrors the Java approach to parsing. 

I built a game class based around storing the game entities as components. There's a toy "system" to move pieces around the board.





 



