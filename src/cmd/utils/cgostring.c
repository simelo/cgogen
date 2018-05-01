#include "cgostring.h"
#include <string.h>
#include <stdlib.h>

GoString_ string_init(const char* p){
	GoString_ s;
	s.size = strlen(p);
	s.str = malloc(s.size + 1);
	strcpy(s.str, p);
	return s;
}

GoString_ string_concat(GoString_ a, GoString_ b){
	GoString_ s;
	s.size = strlen(a.str) + strlen(b.str);
	s.str = malloc(s.size + 1);
	strcpy(s.str, a.str);
	strcat(s.str, b.str);
	return s;
}

unsigned int string_length(GoString_ s){
	return strlen(s.str);
}

char string_charAt(GoString_ s, unsigned int index){
	if( index < strlen(s.str) )
		return s.str[index];
	else
		return 0;
}

GoString_ string_substring(GoString_ s, int index, int len){
	GoString_  result;
	unsigned int length = strlen(s.str);
	if( index + len > length ){
		len = length - index;
	}
	result.size = length;
	result.str = malloc(result.size + 1);
	strcpy(result.str, s.str + index);	
	result.str[len] = 0;
	return result;
}

int string_is_equal(GoString_ a, GoString_ b){
	return strcmp(a.str, b.str) == 0;
}


int string_is_greater(GoString_ a, GoString_ b){
	return strcmp(a.str, b.str) > 0;
}

int string_is_lesser(GoString_ a, GoString_ b){
	return strcmp(a.str, b.str) < 0;
}

int string_is_greater_than_or_equal(GoString_ a, GoString_ b){
	return strcmp(a.str, b.str) >= 0;
}

int string_is_lesser_than_or_equal(GoString_ a, GoString_ b){
	return strcmp(a.str, b.str) <= 0;
}