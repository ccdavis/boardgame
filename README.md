# boardgame

The "aaa.gdf" file has a complete description for the starting state of a strategy board game similar to "Axis and Allies."

The "parser" directory has a complete modern C++ parser for this file along with a stand-alone test program and Makefile.

The format of "aaa.gdf" came about as a way to represent a game state that's machine readable but also very easy to edit with out introducing mistakes in the game description file. At the time I developed it YML and ToML didn't exist; I think I'd use ToML or maybe JSON if I were starting over.

The parser is implemented as a simple recursive descent parser. It's nice as example code showing how to build sucha parser. One advantage it has over a generic YML / JSON parser is that it can very accurately discover mistakes in the data it's parsing and direct you to the exact line with fairly helpful explainations of what's wrong. While not exactly an equivalent , you might think of the parser as a schema to validate the game definition, where the schema is executible code. While changing the parser is harder than updating a schema the recursive descent parser style is relatively easy to understand and modify.


Years ago I wrote  this game with Borland Pascal and intended to build the front-end with Delphi but never did. It ran in text mode mostly as a way to test the game. I got about 90% done and set it aside.


The game setup found in "aaa.gdf" was all in a BP unit  declared with Pascal types. This is actually surprisingly (well, perhaps surprising if you don't know Pascal) easy to read, sort of like Rails' practice of using Ruby for configuration.

Later when I decided to port the game to Java I extracted the game setup from the BP unit and wrote it out in a generically  parsible format.  Using the parser really helped to detect my many mistakes. I never got much beyond the parser with the Java version.

This C++ version pretty much  mirrors the Java approach to parsing. For the rest of the game I'll try out entity component-system architecture which is rather opposite of the original design which was very object-oriented with a strict class hierarchy.






 



