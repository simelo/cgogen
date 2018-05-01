#include "maps.h"
#include <stdio.h>

void MapIntIntSet(GoIntMap* map, int key, int n){
	char strkey[1024]; //big enough for an int
	sprintf(strkey, "%d", key);
	map_set(map, strkey, n);
}

void MapIntStringSet(GoStringMap* map, int key, GoString_ s){
	char strkey[1024]; //big enough for an int
	sprintf(strkey, "%d", key);
	map_set(map, strkey, s);
}

void MapIntObjectSet(GoObjectMap* map, int key, void* s){
	char strkey[1024]; //big enough for an int
	sprintf(strkey, "%d", key);
	map_set(map, strkey, s);
}

void MapStringIntSet(GoIntMap* map, GoString_ key, int n){
	map_set(map, key, n);	
}	
	
void MapStringStringSet(GoStringMap* map, GoString_ key, GoString_ s){
	map_set(map, key, s);
}

void MapStringObjectSet(GoObjectMap* map, GoString_ key, void* s){
	map_set(map, key, s);
}